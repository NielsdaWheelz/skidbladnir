package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/claudeusage"
	"github.com/NielsdaWheelz/skidbladnir/internal/install"
	"github.com/NielsdaWheelz/skidbladnir/internal/quota"
)

// The managed Claude formatter calls this locally, including outside tmux.
func writeClaudeUsage(out, diagnostics io.Writer) error {
	report := quota.ReadClaudeCache(os.Getenv("CLAUDE_CONFIG_DIR"))
	if report.Account == "" {
		return nil
	}
	// The formatter stays local even when every fleet gateway is offline.
	dirs, historyErr := install.Paths()
	if historyErr == nil {
		report, historyErr = claudeusage.Remember(dirs.State, report)
	}
	if historyErr != nil {
		fmt.Fprintln(diagnostics, "claude usage history unavailable")
	}
	var parts []string
	for _, item := range []struct {
		label  string
		window api.ClaudeWindow
	}{
		{"5h", report.FiveHour},
		{"7d", report.SevenDay},
		{"fable", report.FableSevenDay},
	} {
		if amount := item.window.Usage; amount != nil {
			parts = append(parts, item.label+" "+strconv.Itoa(int(amount.UsedPercent))+"%")
		}
	}
	if len(parts) == 0 {
		return nil
	}
	_, err := fmt.Fprintln(out, strings.Join(parts, " · "))
	return err
}
