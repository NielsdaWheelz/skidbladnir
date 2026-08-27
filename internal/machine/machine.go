package machine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	handlePrefix     = "mh-"
	handleRandomSize = 16
	handleTextSize   = len(handlePrefix) + handleRandomSize*2
	handleLineSize   = handleTextSize + 1
)

type Handle struct {
	value string
}

func Parse(value string) (Handle, error) {
	if len(value) != handleTextSize || value[:len(handlePrefix)] != handlePrefix {
		return Handle{}, errors.New("machine handle is not canonical")
	}
	for _, character := range value[len(handlePrefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return Handle{}, errors.New("machine handle is not canonical")
		}
	}
	return Handle{value: value}, nil
}

func (handle Handle) String() string {
	return handle.value
}

func Load(path string) (Handle, error) {
	if path == "" {
		return Handle{}, errors.New("machine handle file path is empty")
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return Handle{}, fmt.Errorf("inspect machine handle file: %w", err)
	}
	if !secureRegularFile(pathInfo.Mode()) {
		return Handle{}, errors.New("machine handle file must be a regular file with mode 0600")
	}
	file, err := os.Open(path)
	if err != nil {
		return Handle{}, fmt.Errorf("open machine handle file: %w", err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close() // justify-ignore-error: the stat failure is authoritative.
		return Handle{}, fmt.Errorf("inspect open machine handle file: %w", err)
	}
	if !secureRegularFile(openedInfo.Mode()) || !os.SameFile(pathInfo, openedInfo) {
		_ = file.Close() // justify-ignore-error: the changed or insecure file is authoritative.
		return Handle{}, errors.New("machine handle file changed while opening")
	}
	contents, err := io.ReadAll(io.LimitReader(file, int64(handleLineSize+1)))
	if err != nil {
		_ = file.Close() // justify-ignore-error: the read failure is authoritative.
		return Handle{}, fmt.Errorf("read machine handle file: %w", err)
	}
	if err := file.Close(); err != nil {
		return Handle{}, fmt.Errorf("close machine handle file: %w", err)
	}
	if len(contents) != handleLineSize || contents[handleTextSize] != '\n' {
		return Handle{}, errors.New("machine handle file has invalid contents")
	}
	handle, err := Parse(string(contents[:handleTextSize]))
	if err != nil {
		return Handle{}, errors.New("machine handle file has invalid contents")
	}
	return handle, nil
}

func Init(path string) (Handle, error) {
	handle, err := Load(path)
	if err == nil {
		return handle, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Handle{}, err
	}

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return Handle{}, fmt.Errorf("create machine handle directory: %w", err)
	}
	random := [handleRandomSize]byte{}
	if _, err := rand.Read(random[:]); err != nil {
		return Handle{}, fmt.Errorf("generate machine handle: %w", err)
	}
	handle, err = Parse(handlePrefix + hex.EncodeToString(random[:]))
	if err != nil {
		panic("generated a noncanonical machine handle") // justify-defect: fixed prefix and lowercase hex close the format.
	}

	temporary, err := os.CreateTemp(parent, ".machine-handle-*")
	if err != nil {
		return Handle{}, fmt.Errorf("create temporary machine handle file: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath) // justify-ignore-error: the original initialization error is authoritative.
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close() // justify-ignore-error: the chmod failure is authoritative.
		return Handle{}, fmt.Errorf("secure temporary machine handle file: %w", err)
	}
	if _, err := io.WriteString(temporary, handle.String()+"\n"); err != nil {
		_ = temporary.Close() // justify-ignore-error: the write failure is authoritative.
		return Handle{}, fmt.Errorf("write temporary machine handle file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close() // justify-ignore-error: the sync failure is authoritative.
		return Handle{}, fmt.Errorf("sync temporary machine handle file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return Handle{}, fmt.Errorf("close temporary machine handle file: %w", err)
	}

	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return Load(path)
		}
		return Handle{}, fmt.Errorf("publish machine handle file: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return Handle{}, fmt.Errorf("remove temporary machine handle file: %w", err)
	}
	removeTemporary = false
	if err := syncDirectory(parent); err != nil {
		return Handle{}, err
	}
	return handle, nil
}

func secureRegularFile(mode fs.FileMode) bool {
	return mode.IsRegular() && mode.Perm() == 0o600 && mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) == 0
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open machine handle directory: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close() // justify-ignore-error: the sync failure is authoritative.
		return fmt.Errorf("sync machine handle directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close machine handle directory: %w", err)
	}
	return nil
}
