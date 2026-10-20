package attention

import "github.com/NielsdaWheelz/skidbladnir/internal/machine"

func (config Config) Valid() bool {
	_, err := machine.Parse(config.ObserverMachine)
	return err == nil
}
