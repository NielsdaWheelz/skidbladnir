package main

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/runtimeenv"
)

// The shell invokes this as a foreground child. Its environment belongs to
// exactly one provider; the parent shell keeps its own account environment.
func agentExec(arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("invalid agent invocation")
	}
	launch, err := agentruntime.DecodeLaunch(arguments[0])
	if err != nil {
		return err
	}
	profileNames := make(map[string]bool, len(launch.Environment))
	for _, variable := range launch.Environment {
		profileNames[variable.Name] = true
	}
	environment := make([]string, 0, len(os.Environ())+len(launch.Environment)+1)
	for _, entry := range runtimeenv.WithoutLaunchContext(os.Environ()) {
		name, _, _ := strings.Cut(entry, "=")
		if !profileNames[name] {
			environment = append(environment, entry)
		}
	}
	for _, variable := range launch.Environment {
		environment = append(environment, variable.Name+"="+variable.Value)
	}
	environment = append(environment, "SKIDBLADNIR_AGENT=1")
	return syscall.Exec(launch.Command, append([]string{launch.Command}, launch.Arguments...), environment)
}
