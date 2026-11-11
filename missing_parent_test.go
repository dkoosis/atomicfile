package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

// Callers test a missing parent with os.IsNotExist, which does not unwrap, so
// WriteFile must return the underlying *PathError unwrapped.
func TestWriteFileMissingParentKeepsIsNotExist(t *testing.T) {
	err := WriteFile(filepath.Join(t.TempDir(), "absent", "out.txt"), []byte("x"), 0o644)
	if !os.IsNotExist(err) {
		t.Fatalf("os.IsNotExist(%v) = false, want true", err)
	}
}
