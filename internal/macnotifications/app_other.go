//go:build !darwin || !cgo

package macnotifications

import (
	"context"
	"errors"
)

func Run(context.Context, string, string) error {
	return errors.New("native notifications require macos with cgo")
}

func DefaultPaths() (string, string, error) {
	return "", "", errors.New("native notifications require macos with cgo")
}

func NativeRegister(string) error {
	return errors.New("native notifications require macos with cgo")
}

func NativeStatus(string) (int, error) {
	return 0, errors.New("native notifications require macos with cgo")
}

func NativeStopApplication(string) error {
	return errors.New("native notifications require macos with cgo")
}
