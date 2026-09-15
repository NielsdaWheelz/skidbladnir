package agentcli

import (
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

func TestTerminalLaunchAndShellSelectors(t *testing.T) {
	ref := fleetclient.Reference{Machine: "mh-11111111111111111111111111111111", TmuxID: "$1", IdentityToken: "lifetime"}.Encode()
	for _, test := range []struct {
		name  string
		args  []string
		valid bool
	}{
		{"terminal", []string{"start", "work", "--machine", "arch", "--terminal"}, true},
		{"agent", []string{"start", "work", "--machine", "arch", "--profile", "personal"}, true},
		{"both launches", []string{"start", "work", "--machine", "arch", "--terminal", "--profile", "personal"}, false},
		{"no launch", []string{"start", "work", "--machine", "arch"}, false},
		{"no name", []string{"start", "--machine", "arch", "--terminal"}, false},
		{"no machine", []string{"start", "work", "--terminal"}, false},
		{"shell name", []string{"shell", "source"}, true},
		{"shell qualified", []string{"shell", "source", "--machine", "arch"}, true},
		{"shell reference", []string{"shell", "--ref", ref, "--json"}, true},
		{"shell name and reference", []string{"shell", "source", "--ref", ref}, false},
		{"shell machine and reference", []string{"shell", "--ref", ref, "--machine", "arch"}, false},
		{"shell cwd override", []string{"shell", "source", "--cwd", "/tmp"}, false},
		{"shell space override", []string{"shell", "source", "--space", "other"}, false},
		{"shell launch override", []string{"shell", "source", "--terminal"}, false},
		{"read terminal", []string{"read", "source", "--terminal"}, true},
		{"send terminal", []string{"send", "source", "literal", "--terminal"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parse(test.args)
			if (err == nil) != test.valid {
				t.Fatalf("command admission: valid=%t want=%t", err == nil, test.valid)
			}
			if err == nil && parsed.request.Ref != "" && parsed.request.Ref != ref {
				t.Fatal("shell parsing replaced the retained reference")
			}
		})
	}
}
