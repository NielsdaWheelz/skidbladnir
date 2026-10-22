// Package privatefile owns reads and durable replacement of private local state.
package privatefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Read returns one private (0600) regular file of at most limit bytes. A missing
// file reports an error that matches os.ErrNotExist.
func Read(path string, limit int) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("private state must be a private regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err = errors.Join(err, file.Close()); err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("private state too large")
	}
	return data, nil
}

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
