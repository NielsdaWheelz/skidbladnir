package fleetclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func terminalHandle(ref Reference) string {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode([]string{"skid-terminal-handle", ref.Machine, ref.TmuxID, ref.IdentityToken})
	digest := sha256.Sum256(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}))
	return "t-" + hex.EncodeToString(digest[:8])
}
func validHandle(value, operation string) bool {
	if len(value) != 18 || strings.Trim(value[2:], "0123456789abcdef") != "" {
		return false
	}
	return strings.HasPrefix(value, "t-") && (operation == "info" || operation == "enter" || operation == "read" || operation == "send" || operation == "text" || operation == "keys" || operation == "wait" || operation == "stop" || operation == "shell" || operation == "group" || operation == "close")
}
