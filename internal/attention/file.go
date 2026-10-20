package attention

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/privatefile"
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
func WriteFile(path string, data []byte) error {
	if err := privatefile.WriteFile(path, data); err != nil {
		return ErrUnavailable
	}
	return nil
}
