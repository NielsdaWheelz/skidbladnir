package agentcontrol

import (
	"strings"
	"unicode/utf8"
)

func validText(text string) bool {
	return text != "" && len(text) <= 32768 && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}
