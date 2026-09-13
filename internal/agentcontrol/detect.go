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
	codexFooter    = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]*(?: (?:none|minimal|low|medium|high|xhigh))?(?: fast)?(?: · (?:~(?:/[^\n]*)?|/[^\n]*))? · context [0-9]+% used(?: · (?:5h|weekly) [0-9]+% left)*(?: · ← for agents)?$`)
	claudeFooter   = regexp.MustCompile(`^(?:⏸ (?:manual|plan) mode|⏵⏵ (?:auto mode|accept edits|bypass permissions|don't ask)) on \(shift\+tab to cycle\)(?: · ← (?:[0-9]{1,2}|99\+) agents?)?$`)
	confirmFooter  = regexp.MustCompile(`^(?:enter to (?:select|confirm)|press enter to confirm)(?: · | or )esc to cancel$`)
)

// Detect uses anchored current interface chrome. Arbitrary titles and quoted
// instructional prose do not establish state; unfamiliar screens stay unknown.
func Detect(provider agentruntime.Provider, text string) agentruntime.Status {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	lastLine := strings.ToLower(strings.TrimSpace(lines[len(lines)-1]))
	choice, dialogFooter, prompt, footer, working := false, false, false, false, false
	trustSelected, trustCancel, trustAllow := false, false, false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		choice = choice || selectedChoice.MatchString(line)
		dialogFooter = dialogFooter || confirmFooter.MatchString(lower)
		working = working || workingChrome.MatchString(lower)
		footer = footer || idleFooter.MatchString(lower)
		switch provider {
		case agentruntime.ProviderCodex:
			prompt = prompt || line == "›" || strings.HasPrefix(line, "› ")
		case agentruntime.ProviderClaude:
			prompt = prompt || line == "❯" || strings.HasPrefix(line, "❯ ")
			trustSelected = trustSelected || lower == "❯ no, exit" || lower == "❯ yes, i trust this folder"
			trustCancel = trustCancel || lower == "no, exit" || lower == "❯ no, exit"
			trustAllow = trustAllow || lower == "yes, i trust this folder" || lower == "❯ yes, i trust this folder"
		}
	}
	if provider == agentruntime.ProviderCodex {
		footer = footer || codexFooter.MatchString(lastLine)
	} else if provider == agentruntime.ProviderClaude {
		footer = footer || claudeFooter.MatchString(lastLine)
	}
	if choice && dialogFooter || trustSelected && trustCancel && trustAllow && confirmFooter.MatchString(lastLine) {
		return agentruntime.Status{State: "blocked", Source: "terminal", Reason: "dialog"}
	}
	if working {
		return agentruntime.Status{State: "working", Source: "terminal"}
	}
	if prompt && footer && !choice && !trustSelected {
		return agentruntime.Status{State: "idle", Source: "terminal"}
	}
	return agentruntime.Status{State: "unknown", Source: "terminal", Reason: "unrecognized"}
}
