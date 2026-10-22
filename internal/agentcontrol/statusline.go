package agentcontrol

import (
	"cmp"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

// The statusline readers read two displayed facts from a composer's already
// anchored footer: quota usage (docs/terminal-usage.md) and the model/effort
// phrase (docs/terminal-model-effort.md). Both are literal text, not account
// reports or execution settings: an unrelated item can imitate either grammar,
// and aliases stay unsupported. Each fact keeps its own admission, so refusing
// one never costs the other, and neither read changes the classifier's
// clipping or rule state.

var (
	codexUsageItem             = regexp.MustCompile(`^(5h|weekly) (0|[1-9][0-9]?|100)% left$`)
	codexTerminalFooterPhrase  = regexp.MustCompile(`^(?:[gG][pP][tT]-[A-Za-z0-9]+(?:[.-][A-Za-z0-9]+)*|codex-auto-review|Codex Auto Review) (?:default|minimal|low|medium|high|xhigh|max|ultra|persistent)(?: Fast)?$`)
	claudeUsageSuffix          = regexp.MustCompile(` │ ctx ` + claudeUsageContext + ` │ ` + claudeUsageWindows + `$`)
	claudeTerminalFooterSuffix = regexp.MustCompile(`^ │ ctx ` + claudeUsageContext + `(?: │ ` + claudeUsageWindows + `)?$`)
)

const (
	claudeUsagePercentage = `(0|[1-9][0-9]?|100)`
	claudeUsageContext    = `(?:(?:0|[1-9][0-9]*)%|--)`
	claudeUsageWindows    = `(?:5h ` + claudeUsagePercentage + `%(?: · 7d ` + claudeUsagePercentage + `%)?|7d ` + claudeUsagePercentage + `%)`
)

// codexReadStatusline reads the ` · `-separated items of F's first row, where
// codex draws its statusline. Usage comes only from complete duration-labelled
// items: upstream renders remaining quota, and monthly, context and arbitrary
// primary or secondary windows cannot supply either account window. The footer
// is the one complete model/effort item.
func codexReadStatusline(line row, width int) (*sessions.TerminalUsage, *sessions.TerminalFooter) {
	text := strings.TrimRight(cellText(line.cells), " ")
	items := strings.Split(strings.TrimLeft(text, " "), " · ")
	usage, usageOK := &sessions.TerminalUsage{}, len(text) < width && !strings.Contains(text, "…")
	var footer *sessions.TerminalFooter
	footerOK := true
	for index, item := range items {
		if codexTerminalFooterPhrase.MatchString(item) {
			// A separator closes an item. Without one, require room at the right
			// edge; byte length conservatively bounds columns after authored text.
			phrase := sessions.TerminalFooter(item)
			footerOK = footerOK && footer == nil && !(index == len(items)-1 && len(text) >= width) && phrase.Valid()
			footer = &phrase
		}
		match := codexUsageItem.FindStringSubmatch(item)
		if match == nil {
			usageOK = usageOK && !strings.HasPrefix(item, "5h ") && !strings.HasPrefix(item, "weekly ")
			continue
		}
		target := &usage.FiveHourUsedPercent
		if match[1] == "weekly" {
			target = &usage.SevenDayUsedPercent
		}
		usageOK = usageOK && *target == nil
		remaining, _ := strconv.Atoi(match[2]) // the grammar admits only canonical 0..100
		used := 100 - remaining
		*target = &used
	}
	if !usageOK || !usage.Valid() {
		usage = nil
	}
	if !footerOK {
		footer = nil
	}
	return usage, footer
}

// claudeReadStatusline reads the managed formatter's final row, immediately
// above the ordinary mode row: usage from its plain text's final suffix, and
// the footer from a parsed row's cells, NBSP kept, as the phrase between its
// first two separators. A candidate on an earlier opaque row refuses its fact,
// as does, for the footer, an unparsed row that could hide one.
func claudeReadStatusline(screen *claudeScreen, bottom, mode int) (*sessions.TerminalUsage, *sessions.TerminalFooter) {
	if mode == bottom+1 {
		return nil, nil // no opaque footer row
	}
	usageOK, footerOK := true, true
	for r := bottom + 1; r < mode-1; r++ {
		line := screen.lines[r]
		_, _, candidate := claudeTerminalFooterFields(strings.TrimRight(cellText(line.cells), " "))
		usageOK = usageOK && !claudeUsageSuffix.MatchString(line.plain)
		footerOK = footerOK && line.presence == rowParsed && !candidate
	}
	line := screen.lines[mode-1]

	var usage *sessions.TerminalUsage
	// Prefix bytes conservatively bound native columns after arbitrary Unicode;
	// the known suffix is ASCII plus the single-cell │ and · separators.
	if match := claudeUsageSuffix.FindStringSubmatch(line.plain); usageOK && match != nil &&
		len(line.plain)-len(match[0])+utf8.RuneCountInString(match[0]) < screen.width {
		percent := func(text string) *int {
			if text == "" {
				return nil
			}
			value, _ := strconv.Atoi(text) // the grammar admits only canonical 0..100
			return &value
		}
		usage = &sessions.TerminalUsage{FiveHourUsedPercent: percent(match[1]), SevenDayUsedPercent: percent(cmp.Or(match[2], match[3]))}
	}

	var footer *sessions.TerminalFooter
	text := strings.TrimRight(cellText(line.cells), " ")
	phrase, suffix, matched := claudeTerminalFooterFields(text)
	// Include every prefix and phrase byte in the conservative column bound.
	if candidate := sessions.TerminalFooter(phrase); footerOK && line.presence == rowParsed && matched &&
		suffix+utf8.RuneCountInString(text[suffix:]) < screen.width && candidate.Valid() {
		footer = &candidate
	}
	return usage, footer
}

func claudeTerminalFooterFields(text string) (phrase string, suffix int, matched bool) {
	fields := strings.Split(text, " │ ")
	if len(fields) != 3 && len(fields) != 4 {
		return "", 0, false
	}
	suffix = len(fields[0]) + len(" │ ") + len(fields[1])
	return fields[1], suffix, claudeTerminalFooterSuffix.MatchString(text[suffix:])
}
