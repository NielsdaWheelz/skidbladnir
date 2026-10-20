// Package privatefile owns durable replacement of private local state.
package privatefile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile replaces one private file. The caller serializes competing writes.
// An error may follow rename: callers must never interpret it as proof that the
// replacement did not happen (in particular, when fencing an authority).
func WriteFile(path string, data []byte) (result error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create private directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".skid-*")
	if err != nil {
		return fmt.Errorf("create private file: %w", err)
	}
	temporary := file.Name()
	defer func() {
		if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, fmt.Errorf("remove temporary private file: %w", err))
		}
	}()
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
