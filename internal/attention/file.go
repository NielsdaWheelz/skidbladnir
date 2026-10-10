package attention

import (
	"errors"
	"os"
	"path/filepath"
)

func StateDirectory() (string, error) {
	directory := os.Getenv("XDG_STATE_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ErrUnavailable
		}
		directory = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(directory) {
		return "", ErrUnavailable
	}
	return filepath.Join(directory, "skidbladnir"), nil
}

// WriteFile durably replaces one private state file. The caller owns serialization.
func WriteFile(path string, data []byte) (result error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ErrUnavailable
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".notifications-*")
	if err != nil {
		return ErrUnavailable
	}
	temporary := file.Name()
	defer func() {
		if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = ErrUnavailable
		}
	}()
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return ErrUnavailable
	} // justify-ignore-error: write failure is primary.
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return ErrUnavailable
	} // justify-ignore-error: sync failure is primary.
	if file.Close() != nil || os.Rename(temporary, path) != nil {
		return ErrUnavailable
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return ErrUnavailable
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return ErrUnavailable
	}
	return nil
}
