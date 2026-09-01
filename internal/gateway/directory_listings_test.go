package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

func TestDirectoryListingsHTTPRequiresAuthorityBeforeReturningTheExactProjection(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "leaf-opaque"), 0o700); err != nil {
		t.Fatalf("create directory fixture: %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, "leaf-opaque", "grandchild"), 0o700); err != nil {
		t.Fatalf("create nested directory fixture: %v", err)
	}
	harness := newDirectoryGatewayHarness(t, home)

	request := harness.request(http.MethodPost, "/v1/directory-listings", `{"directory":"~"}`)
	response := httptest.NewRecorder()
	harness.gateway.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authorized listing status = %d, want 200", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("successful listing omitted its exact content or cache contract")
	}
	want := `{"machine":{"handle":"mh-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","platform":"Darwin"},"directory":"~","children":[{"directory":"~/leaf-opaque","kind":"Directory"}],"omitted":false}` + "\n"
	if response.Body.String() != want {
		t.Fatal("authorized listing did not return the exact machine-bound bytes")
	}
	nestedRequest := harness.request(http.MethodPost, "/v1/directory-listings", `{"directory":"~/leaf-opaque"}`)
	nestedResponse := httptest.NewRecorder()
	harness.gateway.ServeHTTP(nestedResponse, nestedRequest)
	wantNested := `{"machine":{"handle":"mh-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","platform":"Darwin"},"directory":"~/leaf-opaque","parentDirectory":"~","children":[{"directory":"~/leaf-opaque/grandchild","kind":"Directory"}],"omitted":false}` + "\n"
	if nestedResponse.Code != http.StatusOK || nestedResponse.Body.String() != wantNested {
		t.Fatal("nested listing did not return its exact canonical parent projection")
	}
	if !strings.Contains(harness.logs.String(), `"http.route":"/v1/directory-listings"`) || strings.Contains(harness.logs.String(), home) || strings.Contains(harness.logs.String(), "leaf-opaque") {
		t.Fatal("listing logs did not stay on the closed content-free route contract")
	}

	t.Run("authority precedes filesystem access", func(t *testing.T) {
		home := t.TempDir()
		harness := newDirectoryGatewayHarness(t, home)
		if err := os.Remove(home); err != nil {
			t.Fatalf("remove filesystem fixture after service construction: %v", err)
		}
		unauthorized := harness.request(http.MethodPost, "/v1/directory-listings", `{"directory":"~"}`)
		unauthorized.Header.Del("Authorization")
		unauthorizedResponse := httptest.NewRecorder()
		harness.gateway.ServeHTTP(unauthorizedResponse, unauthorized)
		assertDirectoryAPIError(t, unauthorizedResponse, http.StatusUnauthorized, "Unauthenticated", "Authentication required.")

		wrongMachine := harness.request(http.MethodPost, "/v1/directory-listings", `{"directory":"~"}`)
		wrongMachine.Header.Set(machineHeader, "mh-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
		wrongMachineResponse := httptest.NewRecorder()
		harness.gateway.ServeHTTP(wrongMachineResponse, wrongMachine)
		assertDirectoryAPIError(t, wrongMachineResponse, http.StatusConflict, "MachineIdentityMismatch", "The machine identity changed. Fleet reset is required.")
	})
}

func TestDirectoryListingsHTTPRejectsNoncanonicalRequestsAndMapsClosedFailures(t *testing.T) {
	harness := newDirectoryGatewayHarness(t, t.TempDir())
	tests := []struct {
		name            string
		method          string
		path            string
		body            string
		contentEncoding string
		status          int
		code            string
		message         string
	}{
		{name: "missing key", method: http.MethodPost, path: "/v1/directory-listings", body: `{}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "unknown key", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~","extra":false}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "alternate case", method: http.MethodPost, path: "/v1/directory-listings", body: `{"Directory":"~"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "duplicate key", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~","directory":"~/second"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "unpaired escaped surrogate", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~/\ud800"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "null", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":null}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "wrong type", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":[]}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "absolute", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"/tmp"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "relative", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"relative"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "trailing separator", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~/"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "dot segment", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~/./child"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "query", method: http.MethodPost, path: "/v1/directory-listings?cursor=next", body: `{"directory":"~"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "wrong method", method: http.MethodGet, path: "/v1/directory-listings", body: `{"directory":"~"}`, status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "content encoded", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~"}`, contentEncoding: "gzip", status: 400, code: "InvalidRequest", message: "The request is not valid."},
		{name: "unavailable", method: http.MethodPost, path: "/v1/directory-listings", body: `{"directory":"~/missing"}`, status: 422, code: "DirectoryListingUnavailable", message: "This directory cannot be browsed. Enter the path instead."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := harness.request(test.method, test.path, test.body)
			if test.contentEncoding != "" {
				request.Header.Set("Content-Encoding", test.contentEncoding)
			}
			response := httptest.NewRecorder()
			harness.gateway.ServeHTTP(response, request)
			assertDirectoryAPIError(t, response, test.status, test.code, test.message)
		})
	}

	oversized := harness.request(http.MethodPost, "/v1/directory-listings", strings.Repeat(" ", int(MaximumBodyBytes)+1))
	oversizedResponse := httptest.NewRecorder()
	harness.gateway.ServeHTTP(oversizedResponse, oversized)
	assertDirectoryAPIError(t, oversizedResponse, http.StatusRequestEntityTooLarge, "RequestTooLarge", "The request is too large.")
	logs := harness.logs.String()
	if !strings.Contains(logs, `"http.route":"/v1/directory-listings"`) || !strings.Contains(logs, `"skidbladnir.error.code":"DirectoryListingUnavailable"`) {
		t.Fatal("directory failure logs omitted their closed route or error code")
	}
	for _, requestContent := range []string{"cursor=next", "relative", "missing"} {
		if strings.Contains(logs, requestContent) {
			t.Fatal("directory failure logs disclosed request content")
		}
	}
}

func TestDirectoryListingsHTTPRejectsFilesystemAndEncodedBoundsWithoutPartialOutput(t *testing.T) {
	t.Run("filesystem child bound", func(t *testing.T) {
		home := t.TempDir()
		for index := 0; index < 257; index++ {
			if err := os.Mkdir(filepath.Join(home, fmt.Sprintf("child-%03d", index)), 0o700); err != nil {
				t.Fatalf("create bounded directory fixture: %v", err)
			}
		}
		harness := newDirectoryGatewayHarness(t, home)
		response := httptest.NewRecorder()
		harness.gateway.ServeHTTP(response, harness.request(http.MethodPost, "/v1/directory-listings", `{"directory":"~"}`))
		assertDirectoryAPIError(t, response, http.StatusUnprocessableEntity, "DirectoryListingTooLarge", "This directory has too many folders to show. Enter the path instead.")
		if !strings.Contains(harness.logs.String(), `"skidbladnir.error.code":"DirectoryListingTooLarge"`) || strings.Contains(harness.logs.String(), "child-000") {
			t.Fatal("bounded listing log widened beyond the closed error contract")
		}
	})

	t.Run("encoded response bound", func(t *testing.T) {
		home := t.TempDir()
		for index := 0; index < 180; index++ {
			name := fmt.Sprintf("%03d-%s", index, strings.Repeat("<", 100))
			if err := os.Mkdir(filepath.Join(home, name), 0o700); err != nil {
				t.Fatalf("create encoding directory fixture: %v", err)
			}
		}
		harness := newDirectoryGatewayHarness(t, home)
		response := httptest.NewRecorder()
		harness.gateway.ServeHTTP(response, harness.request(http.MethodPost, "/v1/directory-listings", `{"directory":"~"}`))
		assertDirectoryAPIError(t, response, http.StatusUnprocessableEntity, "DirectoryListingTooLarge", "This directory has too many folders to show. Enter the path instead.")
		if strings.Contains(response.Body.String(), "children") {
			t.Fatal("encoded response overflow exposed a partial listing")
		}
	})
}

type directoryGatewayHarness struct {
	gateway *Gateway
	bearer  string
	machine machine.Handle
	logs    *bytes.Buffer
}

func newDirectoryGatewayHarness(t *testing.T, home string) directoryGatewayHarness {
	t.Helper()
	service, err := workdir.New(home)
	if err != nil {
		t.Fatalf("construct workdir service: %v", err)
	}
	bearerPath := filepath.Join(t.TempDir(), "bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("mint bearer: %v", err)
	}
	handle, err := machine.Parse("mh-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("parse machine handle: %v", err)
	}
	logs := &bytes.Buffer{}
	return directoryGatewayHarness{
		gateway: New(Config{
			Workdir:  service,
			Pressure: pressure.NewMonitor(),
			Bearer:   auth.FileVerifier{Path: bearerPath},
			Pairing:  pairing.NewSlot(),
			Logger:   logging.New(logs),
			Machine:  handle,
			Platform: platform.Descriptor{Kind: platform.KindDarwin},
		}),
		bearer:  bearer,
		machine: handle,
		logs:    logs,
	}
}

func (harness directoryGatewayHarness) request(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+harness.bearer)
	request.Header.Set(machineHeader, harness.machine.String())
	return request
}

func assertDirectoryAPIError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("directory API status = %d, want %d", recorder.Code, status)
	}
	var decoded map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode directory API error: %v", err)
	}
	if len(decoded) != 2 || decoded["code"] != code || decoded["message"] != message {
		t.Fatal("directory API error did not match its exact closed contract")
	}
}
