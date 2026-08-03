// Package fsutil provides small filesystem helpers shared across packages
// that need to write files without ever leaving a torn/partial file behind.
package fsutil

import (
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path by creating a temp file in the same
// directory (so the rename is guaranteed atomic on the same filesystem),
// syncing, then renaming over the target. Parent directories are created as
// needed.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed away

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
