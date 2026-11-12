//go:build !darwin

package atomicfile

import "os"

// syncFile forces f to stable storage; plain fsync is the correct barrier on
// non-darwin platforms.
func syncFile(f *os.File) error {
	return f.Sync() //nolint:wrapcheck // thin platform shim; callers add the path context
}
