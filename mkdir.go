package atomicfile

import (
	"os"
	"path/filepath"
)

// mkdirAllSync creates dir and any missing parents (like os.MkdirAll), then
// makes that creation crash-durable by fsyncing every directory whose entry
// gained a newly created child.
//
// The durability reasoning is the whole point of this function. Fsyncing a
// directory persists the names it contains, not its own entry in its parent. A
// newly created directory C is therefore not durable until the directory that
// gained the entry "C" — C's parent — is fsynced. So for a chain created under
// an existing ancestor X (say X/a, X/a/b, X/a/b/c), the set of directories to
// fsync is the parent of each created dir: {X, X/a, X/a/b}. The leaf X/a/b/c is
// deliberately omitted here — the caller's write path already fsyncs
// filepath.Dir(path) == the leaf after the atomic rename, which persists both
// the file entry and (redundantly) the leaf's contents; the leaf's own
// existence is made durable by our fsync of X/a/b.
//
// If dir already exists, nothing is created and nothing is fsynced: the caller's
// existing write path is left byte-for-byte unchanged.
func mkdirAllSync(dir string, perm os.FileMode) error {
	// Walk upward to the deepest existing ancestor, recording the missing chain
	// deepest-first.
	var missing []string
	d := dir
	for {
		info, err := os.Stat(d)
		if err == nil {
			if !info.IsDir() {
				// A path component is a non-directory; let MkdirAll surface the
				// canonical ENOTDIR error below.
				break
			}
			break // deepest existing ancestor found
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, d)

		parent := filepath.Dir(d)
		if parent == d {
			break // reached the filesystem root ("/") or "." — safety net
		}
		d = parent
	}

	// No-op fast path: the directory already exists.
	if len(missing) == 0 {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return err
	}

	// fsync the parent of every created directory, shallowest first. Ordering
	// does not change the durable end state (success is only reported once all
	// fsyncs land), but persisting a parent before its child means any crash
	// mid-sequence leaves a chain rooted at an already-durable ancestor rather
	// than a durable inner directory reachable only through a not-yet-durable
	// outer one.
	for i := len(missing) - 1; i >= 0; i-- {
		if err := syncDir(filepath.Dir(missing[i])); err != nil {
			return err
		}
	}
	return nil
}
