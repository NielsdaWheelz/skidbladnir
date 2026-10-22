package sessions

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// TerminalFooter is literal displayed provider text, not an execution setting.
type TerminalFooter string

func (footer TerminalFooter) Valid() bool {
	text := string(footer)
	return len(text) > 0 && len(text) <= 1024 && utf8.ValidString(text) && strings.TrimSpace(text) == text &&
		!strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) })
}
