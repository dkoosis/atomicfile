package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func TestWithMkdirAll(t *testing.T) {
	t.Run("creates the missing chain and writes", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a", "b", "c", "state.json")

		if err := WriteFile(path, []byte("v1"), 0o640, WithMkdirAll(0o700)); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if string(got) != "v1" {
			t.Errorf("content = %q, want %q", got, "v1")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat file: %v", err)
		}
		if info.Mode().Perm() != 0o640 {
			t.Errorf("file perm = %o, want 640", info.Mode().Perm())
		}
	})

	t.Run("created dirs get the requested perm", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a", "b", "state.json")

		// 0700 is unaffected by the usual 022/002 umasks, so the assertion
		// below is exact rather than umask-dependent.
		if err := WriteFile(path, []byte("x"), 0o644, WithMkdirAll(0o700)); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		for _, dir := range []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b")} {
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("stat %s: %v", dir, err)
			}
			if info.Mode().Perm() != 0o700 {
				t.Errorf("%s perm = %o, want 700", dir, info.Mode().Perm())
			}
		}
	})

	t.Run("existing directory keeps its mode", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "a")
		if err := os.Mkdir(dir, 0o750); err != nil {
			t.Fatalf("seed dir: %v", err)
		}
		path := filepath.Join(dir, "state.json")

		if err := WriteFile(path, []byte("x"), 0o644, WithMkdirAll(0o700)); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o750 {
			t.Errorf("existing dir perm = %o, want 750 preserved", info.Mode().Perm())
		}
	})

	t.Run("errors when an ancestor is a file", func(t *testing.T) {
		root := t.TempDir()
		blocker := filepath.Join(root, "a")
		if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
		path := filepath.Join(blocker, "b", "state.json")

		err := WriteFile(path, []byte("x"), 0o644, WithMkdirAll(0o700))
		if err == nil {
			t.Fatal("want error when an ancestor is a regular file")
		}
		if got, _ := os.ReadFile(blocker); string(got) != "not a dir" {
			t.Errorf("blocking file was clobbered: %q", got)
		}
	})

	t.Run("streamed writes take the option too", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a", "b", "out.txt")

		err := WriteFileFunc(path, 0o644, func(w io.Writer) error {
			_, err := io.WriteString(w, "streamed")
			return err
		}, WithMkdirAll(0o755))
		if err != nil {
			t.Fatalf("WriteFileFunc: %v", err)
		}

		if got, _ := os.ReadFile(path); string(got) != "streamed" {
			t.Errorf("content = %q", got)
		}
	})

	// Two writers racing into the same not-yet-existing chain must both
	// succeed: losing the mkdir race is not an error.
	t.Run("concurrent writers into one new chain", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "a", "b", "c")

		const writers = 8
		errs := make([]error, writers)
		var wg sync.WaitGroup
		for i := range writers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := filepath.Join(dir, fmt.Sprintf("f-%d.txt", i))
				errs[i] = WriteFile(path, []byte("x"), 0o644, WithMkdirAll(0o755))
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Errorf("writer %d: %v", i, err)
			}
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir: %v", err)
		}
		if len(entries) != writers {
			t.Errorf("got %d files, want %d", len(entries), writers)
		}
	})
}

// Without the option the write must fail on a missing parent and leave no
// directories behind — the pre-option behavior, unchanged.
func TestWithoutMkdirAllNothingIsCreated(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a", "b", "state.json")

	if err := WriteFile(path, []byte("x"), 0o644); err == nil {
		t.Fatal("want error for missing parent dir")
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Errorf("directory created without the option (stat err = %v)", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("root not clean: %v", entries)
	}
}

// The durability contract: every directory mkdirAllSync creates must have its
// parent fsynced, shallowest-first, or the new entry can vanish on power loss.
func TestMkdirAllSyncFsyncsEveryCreatedParent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a", "b", "c")

	var synced []string
	record := func(dir string) error {
		synced = append(synced, dir)
		return nil
	}

	if err := mkdirAllSync(target, 0o755, record); err != nil {
		t.Fatalf("mkdirAllSync: %v", err)
	}

	// Created: root/a, root/a/b, root/a/b/c — so their parents, in that order.
	want := []string{root, filepath.Join(root, "a"), filepath.Join(root, "a", "b")}
	if !slices.Equal(synced, want) {
		t.Errorf("fsynced dirs = %v, want %v", synced, want)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Errorf("target dir not created: err = %v", err)
	}
}

func TestMkdirAllSyncSkipsExistingDirs(t *testing.T) {
	root := t.TempDir()

	var synced []string
	record := func(dir string) error {
		synced = append(synced, dir)
		return nil
	}

	if err := mkdirAllSync(root, 0o755, record); err != nil {
		t.Fatalf("mkdirAllSync: %v", err)
	}
	if len(synced) != 0 {
		t.Errorf("fsynced %v for an already-existing dir, want none", synced)
	}
}

// A failed directory fsync must fail the write: reporting success on a
// directory that is not durable is the bug this package exists to prevent.
func TestMkdirAllSyncPropagatesSyncErrors(t *testing.T) {
	root := t.TempDir()
	wantErr := errors.New("fsync refused")

	err := mkdirAllSync(filepath.Join(root, "a", "b"), 0o755, func(string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want the sync error", err)
	}
}
