package install

import (
	"errors"
	"os"
	"path/filepath"
)

type Directories struct{ Home, Config, Data, State, Bin string }

func Paths() (Directories, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Directories{}, err
	}
	dirs := Directories{Home: home, Bin: filepath.Join(home, ".local", "bin")}
	for _, item := range []struct {
		key, fallback string
		target        *string
	}{
		{"XDG_CONFIG_HOME", filepath.Join(home, ".config"), &dirs.Config},
		{"XDG_DATA_HOME", filepath.Join(home, ".local", "share"), &dirs.Data},
		{"XDG_STATE_HOME", filepath.Join(home, ".local", "state"), &dirs.State},
	} {
		value := os.Getenv(item.key)
		if value == "" {
			value = item.fallback
		}
		if !filepath.IsAbs(value) {
			return Directories{}, errors.New(item.key + " must be absolute")
		}
		*item.target = filepath.Join(value, "skidbladnir")
	}
	return dirs, nil
}
