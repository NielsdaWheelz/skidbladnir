package agentcontrol

import (
	"regexp"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

var (
	workingChrome  = regexp.MustCompile(`^[•◦✻✽✶✳✢·] .+\([^\n]*esc to interrupt[^\n]*\)$`)
	selectedChoice = regexp.MustCompile(`^[❯›] [1-9][0-9]*[.)] .+$`)
	idleFooter     = regexp.MustCompile(`^\? for shortcuts(?:\s+[0-9]+% context left)?$`)
	codexFooter    = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]*(?: (?:none|minimal|low|medium|high|xhigh))?(?: fast)?(?: · (?:~(?:/[^\n]*)?|/[^\n]*))? · context [0-9]+% used(?: · (?:5h|weekly) [0-9]+% left)*$`)
)

// Detect uses anchored current interface chrome. Arbitrary titles and quoted
// instructional prose do not establish state; unfamiliar screens stay unknown.
func Detect(provider agentruntime.Provider, text string) agentruntime.Status {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	choice, dialogFooter, prompt, footer, working := false, false, false, false, false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		choice = choice || selectedChoice.MatchString(line)
		dialogFooter = dialogFooter || (strings.HasPrefix(lower, "enter to select") || strings.HasPrefix(lower, "press enter to confirm")) && strings.HasSuffix(lower, "esc to cancel")
		working = working || workingChrome.MatchString(lower)
		footer = footer || idleFooter.MatchString(lower)
		switch provider {
		case agentruntime.ProviderCodex:
			prompt = prompt || line == "›" || strings.HasPrefix(line, "› ")
		case agentruntime.ProviderClaude:
			prompt = prompt || line == "❯" || strings.HasPrefix(line, "❯ ")
		}
	}
	if provider == agentruntime.ProviderCodex {
		footer = footer || codexFooter.MatchString(strings.ToLower(strings.TrimSpace(lines[len(lines)-1])))
	}
	if choice && dialogFooter {
		return agentruntime.Status{State: "blocked", Source: "terminal", Reason: "dialog"}
	}
	if working {
		return agentruntime.Status{State: "working", Source: "terminal"}
	}
	if prompt && footer && !choice {
		return agentruntime.Status{State: "idle", Source: "terminal"}
	}
	return agentruntime.Status{State: "unknown", Source: "terminal", Reason: "unrecognized"}
}
