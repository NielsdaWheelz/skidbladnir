package agentcontrol

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

var codexUsageItem = regexp.MustCompile(`^(5h|weekly) (0|[1-9][0-9]?|100)% left$`)

// codexReadUsage reads only complete duration-labelled native statusline items.
// Upstream renders remaining quota; monthly, context and arbitrary primary or
// secondary windows cannot supply either of these two account windows.
func codexReadUsage(line row, width int) *sessions.TerminalUsage {
	text := strings.TrimRight(cellText(line.cells), " ")
	if line.presence != rowParsed || len(text) >= width || strings.Contains(text, "…") {
		return nil
	}
	usage := &sessions.TerminalUsage{}
	for _, item := range strings.Split(strings.TrimLeft(text, " "), " · ") {
		match := codexUsageItem.FindStringSubmatch(item)
		if match == nil {
			if strings.HasPrefix(item, "5h ") || strings.HasPrefix(item, "weekly ") {
				return nil
			}
			continue
		}
		target := &usage.FiveHourUsedPercent
		if match[1] == "weekly" {
			target = &usage.SevenDayUsedPercent
		}
		if *target != nil {
			return nil
		}
		remaining, err := strconv.Atoi(match[2])
		if err != nil {
			panic("invalid recognized codex quota percentage")
		} // justify-defect: grammar admits canonical integers in 0..100.
		used := 100 - remaining
		*target = &used
	}
	if !usage.Valid() {
		return nil
	}
	return usage
}
