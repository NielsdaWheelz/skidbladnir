package fleet

import (
	"github.com/NielsdaWheelz/skidbladnir/internal/hostconfig"
	"path/filepath"
)

func LoadIdentity(directory string) (Identity, error) {
	var i Identity
	e := readPrivate(filepath.Join(directory, "identity.json"), &i)
	return i, e
}
func LoadHostState(directory string) (HostState, error) {
	var h HostState
	e := readPrivate(filepath.Join(directory, "host-state.json"), &h)
	return h, e
}
func LoadResolved(directory string) (hostconfig.Settings, ApplyStatus, error) {
	var r resolved
	e := readPrivate(filepath.Join(directory, "resolved.json"), &r)
	return r.Settings, r.Application, e
}
