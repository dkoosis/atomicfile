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
