package sessions

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

// TerminalFooter is literal displayed provider text, not an execution setting.
type TerminalFooter string

func (footer TerminalFooter) Valid() bool {
	text := string(footer)
	return len(text) > 0 && len(text) <= 1024 && utf8.ValidString(text) && strings.TrimSpace(text) == text &&
		!strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) })
}

func (footer TerminalFooter) ValidFor(status TerminalStatus) bool {
	return footer.Valid() && successfulTerminalSample(status)
}

func (footer *TerminalFooter) UnmarshalJSON(encoded []byte) error {
	var text string
	if strictjson.Decode(encoded, &text) != nil || !TerminalFooter(text).Valid() {
		return errors.New("invalid terminal footer")
	}
	*footer = TerminalFooter(text)
	return nil
}
