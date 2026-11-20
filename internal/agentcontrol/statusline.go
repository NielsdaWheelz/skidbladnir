package agentcontrol

import (
	"cmp"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

// The statusline readers read two displayed facts from a composer's already
// anchored footer: quota windows and the model/effort phrase (each grammar's
// statusline section, docs/terminal-observation-{codex,claude}.md). Both are
// literal text, not account reports or execution settings: an unrelated item
// can imitate either grammar, and aliases stay unsupported. Each fact keeps its
// own admission, so refusing one never costs the other, and neither read
// changes the classifier's clipping or rule state.

var (
	codexUsageItem             = regexp.MustCompile(`^(5h|weekly) (0|[1-9][0-9]?|100)% left$`)
	codexTerminalFooterPhrase  = regexp.MustCompile(`^(?:[gG][pP][tT]-[A-Za-z0-9]+(?:[.-][A-Za-z0-9]+)*|codex-auto-review|Codex Auto Review) (?:default|minimal|low|medium|high|xhigh|max|ultra|persistent)(?: Fast)?$`)
	claudeUsageSuffix          = regexp.MustCompile(` │ ctx ` + claudeUsageContext + ` │ ` + claudeUsageWindows + `$`)
	claudeTerminalFooterSuffix = regexp.MustCompile(`^ │ ctx ` + claudeUsageContext + `(?: │ ` + claudeUsageWindows + `)?$`)
	claudeResetRow             = regexp.MustCompile(`^resets (?:7d (` + claudeResetInstant + `)(?: · 5h (` + claudeResetInstant + `))?|5h (` + claudeResetInstant + `))$`)
	claudeResetContinuation    = regexp.MustCompile(`(?:^| {3,})(?:(?:resets|7d|5h)(?: |$)|·)`)
)

const (
	claudeUsagePercentage = `(0|[1-9][0-9]?|100)`
	claudeUsageContext    = `(?:(?:0|[1-9][0-9]*)%|--)`
	claudeUsageWindows    = `(?:5h ` + claudeUsagePercentage + `%(?: · 7d ` + claudeUsagePercentage + `%)?|7d ` + claudeUsagePercentage + `%)`
	claudeResetInstant    = `[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} [+-](?:[01][0-9]|2[0-3])[0-5][0-9]`
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

// claudeReadStatusline reads the managed formatter's final row immediately above
// the ordinary mode row. The preceding row may add dates to the admitted usage
// windows. Duplicate candidates refuse their fact independently.
func claudeReadStatusline(screen *claudeScreen, bottom, mode int) (*api.TerminalUsage, *api.TerminalFooter) {
	if mode == bottom+1 {
		return nil, nil // no opaque footer row
	}
	usageOK, footerOK := true, true
	datesOK := bottom < mode-2
	for r := bottom + 1; r < mode-1; r++ {
		line := screen.lines[r]
		text := strings.TrimRight(cellText(line.cells), " ")
		_, _, candidate := claudeTerminalFooterFields(text)
		usageOK = usageOK && !claudeUsageSuffix.MatchString(line.plain)
		footerOK = footerOK && line.presence == rowParsed && !candidate
		if r < mode-2 {
			datesOK = datesOK && line.presence == rowParsed && !strings.HasPrefix(strings.TrimLeft(text, " "), "resets ")
		}
	}
	line := screen.lines[mode-1]
	text := strings.TrimRight(cellText(line.cells), " ")

	var usage *api.TerminalUsage
	// Prefix bytes conservatively bound native columns after arbitrary Unicode;
	// the known suffix is ASCII plus the single-cell │ and · separators.
	if match := claudeUsageSuffix.FindStringSubmatch(line.plain); usageOK && match != nil &&
		len(line.plain)-len(match[0])+utf8.RuneCountInString(match[0]) < screen.width {
		window := func(text string) *api.QuotaWindow {
			if text == "" {
				return nil
			}
			value, _ := strconv.Atoi(text) // the grammar admits only canonical 0..100
			return &api.QuotaWindow{UsedPercent: float64(value)}
		}
		usage = &api.TerminalUsage{FiveHour: window(match[1]), SevenDay: window(cmp.Or(match[2], match[3]))}
		if datesOK {
			claudeReadResets(screen.lines[mode-2].row, screen.width, screen.alternate, usage)
		}
	}

	var footer *api.TerminalFooter
	phrase, suffix, matched := claudeTerminalFooterFields(text)
	// Include every prefix and phrase byte in the conservative column bound.
	if candidate := api.TerminalFooter(phrase); footerOK && line.presence == rowParsed && matched &&
		suffix+utf8.RuneCountInString(text[suffix:]) < screen.width && candidate.Valid() {
		footer = &candidate
	}
	return usage, footer
}

// Fill matching windows only after validating the entire physical row.
func claudeReadResets(line row, width int, alternate bool, usage *api.TerminalUsage) {
	end := len(line.cells)
	for end > 0 && line.cells[end-1].text == " " {
		end--
	}
	if line.presence != rowParsed || end >= width {
		return
	}
	text := strings.TrimLeft(cellText(line.cells[:end]), " ")
	// Classic can share this row with a notification. Like the mode row,
	// the gap is at least three cells. The notification ends two cells in.
	if !alternate && end == width-2 {
		left, right, shared := strings.Cut(text, "   ")
		if shared {
			if claudeResetContinuation.MatchString(strings.TrimLeft(right, " ")) ||
				strings.Contains(right, "…") || strings.Contains(right, "...") {
				return
			}
			text = left
		}
	}
	match := claudeResetRow.FindStringSubmatch(text)
	if match == nil {
		return
	}
	weekly, weeklyOK := parseClaudeReset(match[1])
	fiveHour, fiveHourOK := parseClaudeReset(cmp.Or(match[2], match[3]))
	if !weeklyOK || !fiveHourOK {
		return
	}
	if usage.SevenDay != nil {
		usage.SevenDay.ResetsAt = weekly
	}
	if usage.FiveHour != nil {
		usage.FiveHour.ResetsAt = fiveHour
	}
}

func parseClaudeReset(text string) (*time.Time, bool) {
	if text == "" {
		return nil, true
	}
	parsed, err := time.Parse("2006-01-02 15:04:05 -0700", text)
	if err != nil || parsed.Year() == 0 {
		return nil, false
	}
	utc := parsed.UTC()
	if utc.Year() < 1 || utc.Year() > 9999 {
		return nil, false
	}
	return &utc, true
}

func claudeTerminalFooterFields(text string) (phrase string, suffix int, matched bool) {
	fields := strings.Split(text, " │ ")
	if len(fields) != 3 && len(fields) != 4 {
		return "", 0, false
	}
	suffix = len(fields[0]) + len(" │ ") + len(fields[1])
	return fields[1], suffix, claudeTerminalFooterSuffix.MatchString(text[suffix:])
}
