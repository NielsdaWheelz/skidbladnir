package fleetclient

import (
	"encoding/json"
	"testing"
)

func TestShellResponseUsesOrdinaryObservedSession(t *testing.T) {
	encoded := []byte(`{"observedAt":"2026-09-15T00:00:00Z","session":` + testSession + `}`)
	if !validResponse("shell", encoded, testMachine) {
		t.Fatal("new terminal response rejected the ordinary observed-session contract")
	}
	ref, err := DecodeReference(testRef())
	if err != nil || !(Request{Operation: "shell", Ref: ref.Encode()}).Valid() {
		t.Fatal("new terminal here rejected a retained session reference carrying old agent facts")
	}
	ref.Agent = nil
	if !(Request{Operation: "shell", Ref: ref.Encode()}).Valid() {
		t.Fatal("new terminal here incorrectly requires a foreground agent")
	}
}

func TestCreationFailureDispatchContract(t *testing.T) {
	for _, test := range []struct {
		code, message string
		status        int
	}{
		{"WorkingDirectoryInvalid", "Choose a valid working directory.", 422},
		{"WorkingDirectoryUnavailable", "That directory does not exist or cannot be opened.", 422},
		{"ProfileUnknown", "Choose an available profile.", 422},
		{"SessionNameInvalid", "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.", 422},
		{"SessionNameConflict", "A session with that name already exists.", 409},
		{"ObjectiveInvalid", "Use 1–240 characters without terminal controls.", 422},
	} {
		for _, dispatch := range []string{"not_sent", "unknown"} {
			encoded, _ := json.Marshal(map[string]string{"code": test.code, "message": test.message, "dispatch": dispatch})
			failure := decodeMutationFailure("start", encoded, test.status)
			if dispatch == "not_sent" && (failure == nil || failure.Dispatch != dispatch || failure.Code != test.code) || dispatch == "unknown" && failure != nil {
				t.Fatalf("creation error admission differs from strict host contract: code=%s dispatch=%s", test.code, dispatch)
			}
		}
	}
}
