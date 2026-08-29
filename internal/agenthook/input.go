package agenthook

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

const maximumSessionStartBytes = 64 * 1024

func readSessionStartID(input io.Reader) (string, error) {
	encoded, err := io.ReadAll(io.LimitReader(input, maximumSessionStartBytes+1))
	if err != nil || len(encoded) == 0 || len(encoded) > maximumSessionStartBytes {
		return "", errors.New("read SessionStart hook input")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return "", errors.New("decode SessionStart hook input")
	}
	var sessionID string
	found := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", errors.New("decode SessionStart hook input")
		}
		if key == "session_id" {
			if found || decoder.Decode(&sessionID) != nil {
				return "", errors.New("decode SessionStart hook input")
			}
			found = true
			continue
		}
		var ignored json.RawMessage
		if decoder.Decode(&ignored) != nil {
			return "", errors.New("decode SessionStart hook input")
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return "", errors.New("decode SessionStart hook input")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "", errors.New("decode SessionStart hook input")
	}
	if !found {
		return "", errors.New("SessionStart provider session id is invalid")
	}
	facts, err := agentruntime.NewProviderSessionFacts(sessionID, "")
	if err != nil {
		return "", errors.New("SessionStart provider session id is invalid")
	}
	return facts.ID(), nil
}
