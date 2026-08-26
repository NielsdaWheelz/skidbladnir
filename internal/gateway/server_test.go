package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayListenAddressIsNumericLoopbackOnly(t *testing.T) {
	for _, address := range []string{"127.0.0.1:7341", "[::1]:7341"} {
		if err := ValidateListenAddress(address); err != nil {
			t.Fatalf("loopback address %q rejected: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:7341", "[::]:7341", "192.0.2.1:7341", "localhost:7341", "127.0.0.1:0", "127.0.0.1"} {
		if err := ValidateListenAddress(address); err == nil {
			t.Fatalf("non-loopback or ambiguous address %q accepted", address)
		}
	}
}

func TestTrackedResponseWriterPreservesOptionalHTTPInterfaces(t *testing.T) {
	tracked := &trackedResponseWriter{ResponseWriter: httptest.NewRecorder()}
	if err := http.NewResponseController(tracked).Flush(); err != nil {
		t.Fatalf("ResponseController could not reach the wrapped flusher: %v", err)
	}
}
