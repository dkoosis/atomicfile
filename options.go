package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

// An Option adjusts how a write behaves. Options are opt-in: a call that
// passes none behaves exactly as it did before options existed.
type Option func(*config)

// config is the resolved option set for a single write.
type config struct {
	mkdirAll bool
	dirPerm  os.FileMode
}

// resolve applies opts in order and returns the resulting config.
func resolve(opts []Option) config {
	var c config
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// WithMkdirAll creates the target's parent directory, and any missing
// ancestors, before the write — durably.
//
// This is a durability guarantee, not a convenience. os.MkdirAll returns
// before the new directory entries have reached stable storage, so a power
// loss moments later can take the directory with it even though the write it
// held reported success. That is the same defect one level up from the
// un-fsynced rename this package exists to fix. WithMkdirAll fsyncs the parent
// of every directory it creates, so the whole chain is durable before the file
// write begins.
//
// perm is the mode for directories this option creates, subject to the process
// umask exactly as os.MkdirAll applies it. Directories that already exist keep
// their current mode and are not re-synced.
//
// Without this option the behavior is unchanged: a write into a missing
// directory fails and creates nothing.
//
// Like os.MkdirAll, a failure part-way up the chain leaves the directories
// already created in place; they are not rolled back.
func WithMkdirAll(perm os.FileMode) Option {
	return func(c *config) {
		c.mkdirAll = true
		c.dirPerm = perm
	}
}

// mkdirAllSync creates dir and any missing ancestors, fsyncing the parent of
// every directory it creates so that the new entry is crash-durable. sync is
// the directory-fsync function — syncDir in production, a recorder in tests.
func mkdirAllSync(dir string, perm os.FileMode, sync func(string) error) error {
	missing, err := missingDirs(dir)
	if err != nil {
		return err
	}

	// Create shallowest-first. After each mkdir, fsync the parent directory
	// that now holds the new entry: without that fsync the directory is no
	// more durable than an un-fsynced rename.
	for _, d := range slices.Backward(missing) {
		if err := os.Mkdir(d, perm); err != nil {
			// Tolerate a concurrent creator winning the race, but only if
			// what exists now is really a directory.
			if fi, statErr := os.Stat(d); statErr != nil || !fi.IsDir() {
				return err //nolint:wrapcheck // callers test os.IsNotExist and os.IsPermission, which do not unwrap fmt wrappers
			}
		}
		if err := sync(filepath.Dir(d)); err != nil {
			return err
		}
	}
	return nil
}

// missingDirs walks up from dir and returns the components that do not exist
// yet, deepest first. It fails if an existing component is not a directory.
func missingDirs(dir string) ([]string, error) {
	var missing []string
	for p := dir; ; {
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.IsDir() {
				return nil, &os.PathError{Op: "mkdir", Path: p, Err: syscall.ENOTDIR}
			}
			return missing, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err //nolint:wrapcheck // callers test os.IsNotExist and os.IsPermission, which do not unwrap fmt wrappers
		}
		missing = append(missing, p)
		parent := filepath.Dir(p)
		if parent == p {
			return missing, nil // reached the filesystem root
		}
		p = parent
	}
}
