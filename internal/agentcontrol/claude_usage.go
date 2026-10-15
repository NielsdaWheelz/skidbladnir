package agentcontrol

import (
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

const (
	claudeUsagePercentage = `(0|[1-9][0-9]?|100)`
	claudeUsageContext    = `(?:(?:0|[1-9][0-9]*)%|--)`
	claudeUsageWindows    = `(?:5h ` + claudeUsagePercentage + `%(?: · 7d ` + claudeUsagePercentage + `%)?|7d ` + claudeUsagePercentage + `%)`
)

var claudeUsageSuffix = regexp.MustCompile(` │ ctx ` + claudeUsageContext + ` │ ` + claudeUsageWindows + `$`)

// claudeReadUsage reads the managed formatter's final suffix from an already
// anchored ordinary footer (docs/terminal-usage.md). It never traverses screen
// boundaries or changes the status classifier's clipping/rule state.
func claudeReadUsage(screen *claudeScreen, bottom, mode int) *sessions.TerminalUsage {
	if mode == bottom+1 {
		return nil // no opaque footer row
	}
	for r := bottom + 1; r < mode-1; r++ {
		if claudeUsageSuffix.MatchString(screen.lines[r].plain) {
			return nil // a non-final or duplicated formatter suffix
		}
	}
	text := screen.lines[mode-1].plain
	match := claudeUsageSuffix.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	// Prefix bytes conservatively bound native columns after arbitrary Unicode;
	// the known suffix is ASCII plus the single-cell │ and · separators.
	if len(text)-len(match[0])+utf8.RuneCountInString(match[0]) >= screen.width {
		return nil
	}
	sevenDay := match[2]
	if sevenDay == "" {
		sevenDay = match[3]
	}
	usage := &sessions.TerminalUsage{}
	for _, window := range [...]struct {
		text   string
		target **int
	}{{match[1], &usage.FiveHourUsedPercent}, {sevenDay, &usage.SevenDayUsedPercent}} {
		if window.text == "" {
			continue
		}
		value, err := strconv.Atoi(window.text)
		if err != nil {
			panic("invalid recognized claude usage percentage") // justify-defect: the suffix admits only canonical integers in 0..100.
		}
		*window.target = &value
	}
	return usage
}
