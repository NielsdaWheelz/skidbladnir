package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreationIngressRequiresAnExactLaunchChoice(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want bool
	}{
		{"agent", `{"kind":"agent","profile":"work","cwd":"~"}`, true},
		{"terminal", `{"kind":"terminal","cwd":"~"}`, true},
		{"terminal metadata", `{"kind":"terminal","cwd":"/tmp","optionalTmuxName":"term","objective":"check","space":"work"}`, true},
		{"legacy", `{"profile":"work","cwd":"~"}`, false},
		{"missing profile", `{"kind":"agent","cwd":"~"}`, false},
		{"terminal profile", `{"kind":"terminal","profile":"work","cwd":"~"}`, false},
		{"empty terminal profile", `{"kind":"terminal","profile":"","cwd":"~"}`, false},
		{"unknown kind", `{"kind":"shell","cwd":"~"}`, false},
		{"missing cwd", `{"kind":"terminal"}`, false},
		{"null kind", `{"kind":null,"cwd":"~"}`, false},
		{"null cwd", `{"kind":"terminal","cwd":null}`, false},
		{"duplicate kind", `{"kind":"terminal","kind":"terminal","cwd":"~"}`, false},
		{"unknown field", `{"kind":"terminal","cwd":"~","command":"sh"}`, false},
		{"wrong type", `{"kind":"terminal","cwd":1}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			_, failure := decodeJSON[createSessionRequest](httptest.NewRecorder(), request)
			if (failure == nil) != test.want {
				t.Fatalf("creation admission accepted=%t, want %t", failure == nil, test.want)
			}
		})
	}
}
