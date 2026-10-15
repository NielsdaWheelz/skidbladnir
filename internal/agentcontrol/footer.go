package agentcontrol

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var claudeTerminalFooterSuffix = regexp.MustCompile(`^ │ ctx ` + claudeUsageContext + `(?: │ ` + claudeUsageWindows + `)?$`)

var codexTerminalFooterPhrase = regexp.MustCompile(`^(?:[gG][pP][tT]-[A-Za-z0-9]+(?:[.-][A-Za-z0-9]+)*|codex-auto-review|Codex Auto Review) (?:default|minimal|low|medium|high|xhigh|max|ultra|persistent)(?: Fast)?$`)

// codexReadTerminalFooter recognizes one complete model/effort item in the
// composer-anchored footer. This is displayed text, not an execution setting: an
// unrelated item can imitate the grammar, and aliases remain unsupported.
func codexReadTerminalFooter(line row, width int) *sessions.TerminalFooter {
	text := strings.TrimRight(cellText(line.cells), " ")
	items := strings.Split(strings.TrimLeft(text, " "), " · ")
	var footer *sessions.TerminalFooter
	for index, item := range items {
		if !codexTerminalFooterPhrase.MatchString(item) {
			continue
		}
		// A separator closes an item. Without one, require room at the right
		// edge; byte length conservatively bounds columns after authored text.
		if footer != nil || index == len(items)-1 && len(text) >= width {
			return nil
		}
		phrase := sessions.TerminalFooter(item)
		if !phrase.Valid() {
			return nil
		}
		footer = &phrase
	}
	return footer
}

// claudeReadTerminalFooter reads one complete managed phrase immediately above
// the ordinary mode row. It shares quota productions, not quota admission.
func claudeReadTerminalFooter(screen *claudeScreen, bottom, mode int) *sessions.TerminalFooter {
	if mode == bottom+1 {
		return nil
	}
	for r := bottom + 1; r < mode-1; r++ {
		line := screen.lines[r]
		if line.presence != rowParsed {
			return nil // unreadable opaque text could hide another candidate
		}
		if _, _, matched := claudeTerminalFooterFields(strings.TrimRight(cellText(line.cells), " ")); matched {
			return nil
		}
	}
	line := screen.lines[mode-1]
	if line.presence != rowParsed {
		return nil
	}
	text := strings.TrimRight(cellText(line.cells), " ")
	phrase, suffix, matched := claudeTerminalFooterFields(text)
	if !matched {
		return nil
	}
	// Include every prefix and phrase byte in the conservative column bound.
	if suffix+utf8.RuneCountInString(text[suffix:]) >= screen.width {
		return nil
	}
	footer := sessions.TerminalFooter(phrase)
	if !footer.Valid() {
		return nil
	}
	return &footer
}

func claudeTerminalFooterFields(text string) (phrase string, suffix int, matched bool) {
	fields := strings.Split(text, " │ ")
	if len(fields) != 3 && len(fields) != 4 {
		return "", 0, false
	}
	suffix = len(fields[0]) + len(" │ ") + len(fields[1])
	return fields[1], suffix, claudeTerminalFooterSuffix.MatchString(text[suffix:])
}
