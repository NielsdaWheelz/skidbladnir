package machine_test

import (
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
)

func TestParseAcceptsOnlyCanonicalHandles(t *testing.T) {
	const canonical = "mh-0123456789abcdef0123456789abcdef"
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "canonical", value: canonical, valid: true},
		{name: "empty", value: ""},
		{name: "missing prefix", value: "0123456789abcdef0123456789abcdef"},
		{name: "short", value: "mh-0123456789abcdef0123456789abcde"},
		{name: "long", value: "mh-0123456789abcdef0123456789abcdef0"},
		{name: "uppercase", value: "mh-0123456789ABCDEF0123456789ABCDEF"},
		{name: "nonhex", value: "mh-0123456789abcdef0123456789abcdeg"},
		{name: "newline", value: canonical + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handle, err := machine.Parse(test.value)
			if test.valid {
				if err != nil || handle.String() != test.value {
					t.Fatal("canonical machine handle was rejected or changed")
				}
				return
			}
			if err == nil {
				t.Fatal("noncanonical machine handle was accepted")
			}
		})
	}
}
