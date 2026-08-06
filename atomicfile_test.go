package atomicfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriteFile(t *testing.T) {
	t.Run("creates new file with perm", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")

		if err := WriteFile(path, []byte("v1"), 0o640); err != nil {
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
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o640 {
			t.Errorf("perm = %o, want 640", info.Mode().Perm())
		}
	})

	t.Run("replace preserves existing perms", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")
		if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}

		// perm arg deliberately differs; the existing 0600 must win.
		if err := WriteFile(path, []byte("new"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("perm = %o, want existing 600 preserved", info.Mode().Perm())
		}
		if got, _ := os.ReadFile(path); string(got) != "new" {
			t.Errorf("content = %q, want %q", got, "new")
		}
	})

	t.Run("missing parent dir errors and leaves nothing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-such", "state.json")

		if err := WriteFile(path, []byte("x"), 0o644); err == nil {
			t.Fatal("want error for missing parent dir")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("dir not clean after failed write: %v", entries)
		}
	})
}

func TestWriteFileFunc(t *testing.T) {
	t.Run("streams body", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.txt")

		err := WriteFileFunc(path, 0o644, func(w io.Writer) error {
			for i := range 3 {
				if _, err := fmt.Fprintf(w, "chunk-%d\n", i); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WriteFileFunc: %v", err)
		}

		got, _ := os.ReadFile(path)
		if string(got) != "chunk-0\nchunk-1\nchunk-2\n" {
			t.Errorf("content = %q", got)
		}
	})

	t.Run("fn error leaves target untouched and no temp litter", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}

		wantErr := fmt.Errorf("body failed")
		err := WriteFileFunc(path, 0o644, func(io.Writer) error { return wantErr })
		if err != wantErr { //nolint:errorlint // the exact error must pass through unwrapped
			t.Fatalf("err = %v, want the fn error verbatim", err)
		}

		if got, _ := os.ReadFile(path); string(got) != "old" {
			t.Errorf("target changed on failed write: %q", got)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Errorf("temp file littered: %v", entries)
		}
	})
}

func TestWriteFile_WithMkdirAll(t *testing.T) {
	t.Run("creates nested missing parents", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a", "b", "c", "state.json")

		if err := WriteFile(path, []byte("v1"), 0o640, WithMkdirAll()); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if string(got) != "v1" {
			t.Errorf("content = %q, want %q", got, "v1")
		}
		for _, d := range []string{"a", "a/b", "a/b/c"} {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(d)))
			if err != nil {
				t.Fatalf("stat %s: %v", d, err)
			}
			if !info.IsDir() {
				t.Errorf("%s is not a directory", d)
			}
		}
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
			t.Errorf("perm = %o, want 640", info.Mode().Perm())
		}
	})

	t.Run("already-exists parent is a no-op", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "state.json")

		synced := captureSyncDir(t)

		if err := WriteFile(path, []byte("v1"), 0o644, WithMkdirAll()); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if got, _ := os.ReadFile(path); string(got) != "v1" {
			t.Errorf("content = %q", got)
		}
		// Only the write-path fsync of the target's own dir — mkdir added none.
		if want := []string{root}; !equalStrings(*synced, want) {
			t.Errorf("synced dirs = %v, want %v (no mkdir fsyncs)", *synced, want)
		}
	})

	t.Run("fsyncs each created dir chain top-down, then the write dir", func(t *testing.T) {
		x := t.TempDir()
		path := filepath.Join(x, "a", "b", "c", "state.json")

		synced := captureSyncDir(t)

		if err := WriteFile(path, []byte("v1"), 0o644, WithMkdirAll()); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		// mkdir fsyncs the PARENT of each created dir, shallowest first:
		// {x (gained a), x/a (gained b), x/a/b (gained c)}; then the existing
		// write path fsyncs x/a/b/c (gained the file). The leaf is never
		// double-fsynced by mkdir.
		want := []string{
			x,
			filepath.Join(x, "a"),
			filepath.Join(x, "a", "b"),
			filepath.Join(x, "a", "b", "c"),
		}
		if !equalStrings(*synced, want) {
			t.Errorf("synced dirs = %v,\n           want %v", *synced, want)
		}
	})

	t.Run("streams through WriteFileFunc too", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a", "b", "out.txt")

		err := WriteFileFunc(path, 0o644, func(w io.Writer) error {
			_, err := io.WriteString(w, "streamed")
			return err
		}, WithMkdirAll())
		if err != nil {
			t.Fatalf("WriteFileFunc: %v", err)
		}
		if got, _ := os.ReadFile(path); string(got) != "streamed" {
			t.Errorf("content = %q", got)
		}
	})

	t.Run("write failure keeps created dirs and leaves no temp litter", func(t *testing.T) {
		// Documented contract: the mkdir path does NOT roll back directories it
		// created, and a failed write leaves no temp-file litter inside the new
		// leaf. Locks both halves so a future refactor can't silently change them.
		root := t.TempDir()
		leaf := filepath.Join(root, "a", "b")
		path := filepath.Join(leaf, "state.json")

		wantErr := fmt.Errorf("body failed")
		err := WriteFileFunc(path, 0o644, func(io.Writer) error { return wantErr }, WithMkdirAll())
		if err != wantErr { //nolint:errorlint // the exact fn error must pass through unwrapped
			t.Fatalf("err = %v, want the fn error verbatim", err)
		}
		// Created dirs remain (no rollback).
		if info, err := os.Stat(leaf); err != nil || !info.IsDir() {
			t.Errorf("created dir %s missing after failed write (err=%v)", leaf, err)
		}
		// No temp litter inside the new leaf.
		entries, _ := os.ReadDir(leaf)
		if len(entries) != 0 {
			t.Errorf("temp litter in created dir: %v", entries)
		}
	})

	t.Run("without the option a missing parent still errors", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-such", "state.json")

		if err := WriteFile(path, []byte("x"), 0o644); err == nil {
			t.Fatal("want error for missing parent dir without WithMkdirAll")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Errorf("dir not clean after failed default write: %v", entries)
		}
	})
}

// captureSyncDir wraps the package's syncDir seam to record, in call order,
// every directory fsynced during the test, restoring the original at cleanup.
func captureSyncDir(t *testing.T) *[]string {
	t.Helper()
	orig := syncDir
	t.Cleanup(func() { syncDir = orig })
	var seen []string
	syncDir = func(dir string) error {
		seen = append(seen, dir)
		return orig(dir)
	}
	return &seen
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Concurrent readers must only ever observe a complete old or complete new
// body — never a torn mix — while writers replace the file repeatedly.
func TestReplaceIsAtomicUnderConcurrentReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.bin")
	body := func(n int) []byte {
		b := make([]byte, 0, 4096)
		for range 512 {
			b = append(b, fmt.Sprintf("%04d,", n)...)
		}
		return b
	}
	if err := WriteFile(path, body(0), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const rounds = 50
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= rounds; i++ {
			if err := WriteFile(path, body(i), 0o644); err != nil {
				t.Errorf("writer round %d: %v", i, err)
				return
			}
		}
	}()

	for range 200 {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if len(got)%5 != 0 || len(got) == 0 {
			t.Fatalf("torn read: %d bytes", len(got))
		}
		first := string(got[:5])
		for off := 0; off < len(got); off += 5 {
			if string(got[off:off+5]) != first {
				t.Fatalf("torn read: mixed bodies %q vs %q at %d", first, got[off:off+5], off)
			}
		}
	}
	wg.Wait()
}
