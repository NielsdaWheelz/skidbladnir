package main

import (
	"encoding/base64"
	"errors"
	"os"
)

// agentExec writes the launch's command text, base64url without padding, for
// the initialized destination shell to evaluate exactly once. It decodes data
// only: it never launches a provider or changes its environment.
func agentExec(arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("invalid agent invocation")
	}
	command, err := base64.RawURLEncoding.Strict().DecodeString(arguments[0])
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(command)
	return err
}
