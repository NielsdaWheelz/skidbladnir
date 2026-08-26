package sessions

import (
	"bytes"
	"strings"
	"testing"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func TestSessionIdentityTokenBindsServerLifetimeAndSessionID(t *testing.T) {
	server := tmuxclient.ServerIdentity{
		Epoch: "v1-0123456789abcdef0123456789abcdef", PID: "1234", StartTime: "1720000000",
	}
	token, err := makeIdentityToken(server, "$42")
	if err != nil {
		t.Fatalf("make identity token: %v", err)
	}
	if token != server.Epoch+".1234.1720000000.42" {
		t.Fatalf("identity token = %q", token)
	}
	parsed, ok := parseIdentityToken(token, "$42")
	if !ok || parsed != server {
		t.Fatalf("parse identity token = %+v, %t", parsed, ok)
	}
	if _, ok := parseIdentityToken(token, "$43"); ok {
		t.Fatal("identity token was accepted for another session id")
	}
	if _, ok := parseIdentityToken(strings.Replace(token, "v1-", "v2-", 1), "$42"); ok {
		t.Fatal("identity token was accepted for another version")
	}
	if _, ok := parseIdentityToken(strings.Replace(token, ".1234.", ".0.", 1), "$42"); ok {
		t.Fatal("identity token was accepted for an invalid server PID")
	}
}

func TestServerEpochFromEntropyIsClosedAndDeterministic(t *testing.T) {
	epoch, err := serverEpochFromEntropy(bytes.NewReader([]byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}))
	if err != nil {
		t.Fatalf("mint server epoch: %v", err)
	}
	const want = "v1-00112233445566778899aabbccddeeff"
	if epoch != want || !serverEpochPattern.MatchString(epoch) {
		t.Fatalf("server epoch = %q, want %q", epoch, want)
	}
	if _, err := serverEpochFromEntropy(bytes.NewReader(make([]byte, 15))); err == nil {
		t.Fatal("short entropy unexpectedly minted a server epoch")
	}
}
