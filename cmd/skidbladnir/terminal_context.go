package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/NielsdaWheelz/skidbladnir/internal/process"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
)

func terminalContext(arguments []string) error {
	switch {
	case len(arguments) == 1 && arguments[0] == "new":
		_, err := fmt.Fprintln(os.Stdout, terminalcontext.NewConnectionID())
		return err
	case len(arguments) == 3 && arguments[0] == "register":
		pid, err := strconv.Atoi(arguments[2])
		if err != nil || pid <= 0 || strconv.Itoa(pid) != arguments[2] {
			return errors.New("invalid shell pid")
		}
		return terminalcontext.Register(arguments[1], process.PID(pid))
	default:
		return errors.New("invalid terminal context invocation")
	}
}
