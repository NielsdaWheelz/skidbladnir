package agentcontrol

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

// The screen grammars admit literal model/effort phrases for both providers
// and displayed quota windows for Codex. None affect terminal classification.
const (
	claudePercent        = `(?:0|[1-9][0-9]?|100)%`
	claudeGeneralWindows = `(?:5h ` + claudePercent + `(?: · 7d ` + claudePercent + `)?|7d ` + claudePercent + `)`
	claudeFableWindow    = `fable ` + claudePercent
)

var (
	codexUsageItem             = regexp.MustCompile(`^(5h|weekly) (0|[1-9][0-9]?|100)% left$`)
	codexTerminalFooterPhrase  = regexp.MustCompile(`^(?:[gG][pP][tT]-[A-Za-z0-9]+(?:[.-][A-Za-z0-9]+)*|codex-auto-review|Codex Auto Review) (?:default|minimal|low|medium|high|xhigh|max|ultra|persistent)(?: Fast)?$`)
	claudeTerminalFooterSuffix = regexp.MustCompile(`^ │ ctx (?:(?:0|[1-9][0-9]*)%|--)(?: │ (?:` + claudeGeneralWindows + `(?: · ` + claudeFableWindow + `)?|` + claudeFableWindow + `))?$`)
)

// codexReadStatusline reads the ` · `-separated items of F's first row, where
// codex draws its statusline. Usage comes only from complete duration-labelled
// items: upstream renders remaining quota, and monthly, context and arbitrary
// primary or secondary windows cannot supply either account window. The footer
// is the one complete model/effort item.
func codexReadStatusline(line row, width int) (*api.TerminalUsage, *api.TerminalFooter) {
	text := strings.TrimRight(cellText(line.cells), " ")
	items := strings.Split(strings.TrimLeft(text, " "), " · ")
	usage, usageOK := &api.TerminalUsage{}, len(text) < width && !strings.Contains(text, "…")
	var footer *api.TerminalFooter
	footerOK := true
	for index, item := range items {
		if codexTerminalFooterPhrase.MatchString(item) {
			// A separator closes an item. Without one, require room at the right
			// edge; byte length conservatively bounds columns after authored text.
			phrase := api.TerminalFooter(item)
			footerOK = footerOK && footer == nil && !(index == len(items)-1 && len(text) >= width) && phrase.Valid()
			footer = &phrase
		}
		match := codexUsageItem.FindStringSubmatch(item)
		if match == nil {
			usageOK = usageOK && !strings.HasPrefix(item, "5h ") && !strings.HasPrefix(item, "weekly ")
			continue
		}
		target := &usage.FiveHour
		if match[1] == "weekly" {
			target = &usage.SevenDay
		}
		usageOK = usageOK && *target == nil
		remaining, _ := strconv.Atoi(match[2]) // the grammar admits only canonical 0..100
		*target = &api.QuotaWindow{UsedPercent: float64(100 - remaining)}
	}
	if !usageOK || usage.FiveHour == nil && usage.SevenDay == nil {
		usage = nil
	}
	if !footerOK {
		footer = nil
	}
	return usage, footer
}

// The managed model/effort phrase sits immediately above the ordinary mode
// row. An earlier candidate or an unparsed opaque row makes it ambiguous.
func claudeReadStatusline(screen *claudeScreen, bottom, mode int) *api.TerminalFooter {
	if mode == bottom+1 {
		return nil
	}
	for r := bottom + 1; r < mode-1; r++ {
		line := screen.lines[r]
		_, _, candidate := claudeTerminalFooterFields(strings.TrimRight(cellText(line.cells), " "))
		if line.presence != rowParsed || candidate {
			return nil
		}
	}
	line := screen.lines[mode-1]
	text := strings.TrimRight(cellText(line.cells), " ")
	phrase, suffix, matched := claudeTerminalFooterFields(text)
	candidate := api.TerminalFooter(phrase)
	// Count every prefix and phrase byte as a cell to bound arbitrary Unicode.
	// The suffix's separators are single-cell glyphs.
	if line.presence != rowParsed || !matched || suffix+utf8.RuneCountInString(text[suffix:]) >= screen.width || !candidate.Valid() {
		return nil
	}
	return &candidate
}

func claudeTerminalFooterFields(text string) (phrase string, suffix int, matched bool) {
	fields := strings.Split(text, " │ ")
	if len(fields) != 3 && len(fields) != 4 {
		return "", 0, false
	}
	suffix = len(fields[0]) + len(" │ ") + len(fields[1])
	return fields[1], suffix, claudeTerminalFooterSuffix.MatchString(text[suffix:])
}
