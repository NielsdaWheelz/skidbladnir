package sessions

import (
	"bytes"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/catalog"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func TestCharacterSelectionChoosesALeastUsedStableWinner(t *testing.T) {
	characters := []catalog.Character{
		{Key: "norse.alpha", DisplayName: "Alpha"},
		{Key: "norse.beta", DisplayName: "Beta"},
		{Key: "norse.gamma", DisplayName: "Gamma"},
	}
	usage := map[string]int{
		"norse.alpha": 2,
		"norse.beta":  0,
		"norse.gamma": 1,
	}
	if got := selectCharacter(characters, usage, "v1-test\x00$7"); got.Key != "norse.beta" {
		t.Fatalf("least-used character = %q, want norse.beta", got.Key)
	}

	for key := range usage {
		usage[key] = 0
	}
	first := selectCharacter(characters, usage, "v1-test\x00$7")
	reversed := []catalog.Character{characters[2], characters[1], characters[0]}
	second := selectCharacter(reversed, usage, "v1-test\x00$7")
	if first.Key != "norse.gamma" || second != first {
		t.Fatalf("stable tied selection = (%+v, %+v), want norse.gamma independent of catalogue order", first, second)
	}
}

func TestCharacterSelectionBalancesCommittedSequentialAssignments(t *testing.T) {
	characters := []catalog.Character{
		{Key: "norse.alpha", DisplayName: "Alpha"},
		{Key: "norse.beta", DisplayName: "Beta"},
		{Key: "norse.gamma", DisplayName: "Gamma"},
	}
	usage := map[string]int{}
	for _, id := range []string{"$1", "$2", "$3", "$4", "$5", "$6"} {
		selected := selectCharacter(characters, usage, "v1-test\x00"+id)
		usage[selected.Key]++
	}
	for _, character := range characters {
		if usage[character.Key] != 2 {
			t.Fatalf("balanced usage[%q] = %d, want 2: %+v", character.Key, usage[character.Key], usage)
		}
	}
}

func TestGeneratedTmuxNamesUseIndependentProfileNamespaces(t *testing.T) {
	names := map[string]struct{}{
		"skidbladnir-work-1":     {},
		"skidbladnir-personal-1": {},
	}
	if got := generatedTmuxName(names, "work"); got != "skidbladnir-work-2" {
		t.Fatalf("generated work tmux name = %q, want skidbladnir-work-2", got)
	}
	if got := generatedTmuxName(names, "claude-work"); got != "skidbladnir-claude-work-1" {
		t.Fatalf("generated Claude tmux name = %q, want skidbladnir-claude-work-1", got)
	}
}

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
