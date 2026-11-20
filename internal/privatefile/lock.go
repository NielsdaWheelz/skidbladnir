package privatefile

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Lock waits for exclusive ownership of a stable lock file. Close releases it.
// The lock path must be separate from the state file that gets replaced.
func Lock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}
