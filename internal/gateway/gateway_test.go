package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIngressAdmitsExactlyOneUnambiguousJSONDocument(t *testing.T) {
	const valid = `{"cwd":"/src","profile":"work","optionalTmuxName":"card","objective":"ship"}`
	tests := []struct {
		name     string
		body     string
		accepted bool
	}{
		{name: "exact document", body: valid, accepted: true},
		{name: "duplicate member", body: `{"cwd":"/src","profile":"work","optionalTmuxName":"card","objective":"ship","cwd":"/etc"}`},
		{name: "wrong-case member", body: `{"CWD":"/src","profile":"work","optionalTmuxName":"card","objective":"ship"}`},
		{name: "case-alias duplicate member", body: `{"cwd":"/src","CWD":"/etc","profile":"work","optionalTmuxName":"card","objective":"ship"}`},
		{name: "duplicate member inside a nested value", body: `{"cwd":{"a":1,"a":2},"profile":"work","optionalTmuxName":"card","objective":"ship"}`},
		{name: "unknown member", body: `{"cwd":"/src","profile":"work","optionalTmuxName":"card","objective":"ship","extra":"value"}`},
		{name: "trailing document", body: valid + valid},
		{name: "trailing text", body: valid + " garbage"},
		{name: "invalid UTF-8", body: "{\"cwd\":\"\xff\",\"profile\":\"work\",\"optionalTmuxName\":\"card\",\"objective\":\"ship\"}"},
		{name: "null literal", body: `null`},
		{name: "empty body", body: ``},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			decoded, failure := decodeJSON[createSessionRequest](httptest.NewRecorder(), request)
			if test.accepted {
				if failure != nil {
					t.Fatalf("exact request was rejected as %q", failure.Code)
				}
				if decoded.CWD.value != "/src" || decoded.Profile.value != "work" {
					t.Fatalf("exact request decoded to %+v", decoded)
				}
				return
			}
			if failure == nil {
				t.Fatalf("ambiguous request body was accepted as %+v", decoded)
			}
			if failure.Code != errorInvalidRequest.Code {
				t.Fatalf("ambiguous request failed as %q, want %q", failure.Code, errorInvalidRequest.Code)
			}
		})
	}
}

func TestRequestIngressRejectsABodyBeyondItsCap(t *testing.T) {
	body := `{"cwd":"` + strings.Repeat("a", int(MaximumBodyBytes)) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = -1
	_, failure := decodeJSON[createSessionRequest](httptest.NewRecorder(), request)
	if failure == nil || failure.Code != errorRequestTooLarge.Code {
		t.Fatalf("oversize body failed as %+v, want %q", failure, errorRequestTooLarge.Code)
	}
}
