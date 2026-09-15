package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

func TestSpaceCreateIngressAcceptsOptionalCanonicalLabel(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"cwd":"/src","profile":"work","space":"alpha"}`))
	request.Header.Set("Content-Type", "application/json")
	_, failure := decodeJSON[createSessionRequest](httptest.NewRecorder(), request)
	if failure != nil {
		t.Fatalf("valid assigned create rejected: code=%s", failure.Code)
	}
}

func TestSpaceMutationIngressRequiresExactlyTwoNonNullStrings(t *testing.T) {
	for _, test := range []struct {
		name, body string
		accepted   bool
	}{
		{"assign", `{"identityToken":"token","space":"alpha"}`, true},
		{"clear", `{"identityToken":"token","space":""}`, true},
		{"missing space", `{"identityToken":"token"}`, false},
		{"missing token", `{"space":"alpha"}`, false},
		{"null space", `{"identityToken":"token","space":null}`, false},
		{"null token", `{"identityToken":null,"space":"alpha"}`, false},
		{"wrong type", `{"identityToken":"token","space":1}`, false},
		{"extra field", `{"identityToken":"token","space":"alpha","tmuxName":"name"}`, false},
		{"wrong case", `{"identityToken":"token","Space":"alpha"}`, false},
		{"duplicate", `{"identityToken":"token","space":"alpha","space":"beta"}`, false},
		{"unpaired surrogate", `{"identityToken":"token","space":"\ud800"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v1/sessions/$7/space", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			decoded, failure := decodeJSON[setSessionSpaceRequest](httptest.NewRecorder(), request)
			if (failure == nil) != test.accepted {
				t.Fatal("membership request acceptance differs from exact field contract")
			}
			if failure == nil && (!decoded.Space.present || !decoded.IdentityToken.present) {
				t.Fatal("accepted mutation lost field presence")
			}
		})
	}
}

func TestSpaceProjectionOmitsOnlyUnassignedMembership(t *testing.T) {
	for _, text := range []string{"", "alpha"} {
		label, err := space.Parse(text)
		if err != nil {
			t.Fatal("construct canonical membership")
		}
		card, err := mapSession(sessions.Session{TmuxID: "$7", TmuxName: "session", IdentityToken: "identity", Character: catalog.Character{Key: "norse.durinn", DisplayName: "Durinn"}, Space: label}, nil)
		if err != nil {
			t.Fatal("project valid session")
		}
		encoded, err := json.Marshal(card)
		if err != nil {
			t.Fatal("encode valid session")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal("decode projection fields")
		}
		value, present := fields["space"]
		if present != (text != "") || present && string(value) != `"alpha"` {
			t.Fatal("optional membership was null, empty, absent, or changed incorrectly")
		}
	}
}

func TestSpaceLogRouteClosesSessionIdentity(t *testing.T) {
	if got := requestRoute("/v1/sessions/$7/space"); got != logging.Route("/v1/sessions/{tmuxId}/space") {
		t.Fatalf("space route is not normalized: route=%s", got)
	}
	if _, err := logging.NewRequestCompleted(logging.Method("PUT"), logging.Route("/v1/sessions/{tmuxId}/space"), http.StatusUnprocessableEntity, 0, logging.ErrorCode("SpaceInvalid")); err != nil {
		t.Fatal("space request completion is outside the closed log contract")
	}
}
