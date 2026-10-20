package main

import (
	"errors"
	"io"
	"os"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

// The initialized destination shell evaluates this opaque text exactly once.
// This child decodes data only; it never launches a provider or changes its environment.
func agentExec(arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("invalid agent invocation")
	}
	launch, err := agentruntime.DecodeLaunch(arguments[0])
	if err != nil {
		return err
	}
	_, err = io.WriteString(os.Stdout, launch.Command)
	return err
}
