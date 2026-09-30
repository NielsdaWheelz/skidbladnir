package agentcontrol

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

var (
	workingChrome     = regexp.MustCompile(`^[•◦✻✽✶✳✢·] .+\([^\n]*esc to interrupt[^\n]*\)$`)
	selectedChoice    = regexp.MustCompile(`^[❯›] [1-9][0-9]*[.)] .+$`)
	idleFooter        = regexp.MustCompile(`^\? for shortcuts(?:\s+[0-9]+% context left)?$`)
	codexFooter       = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]*(?: (?:default|none|minimal|low|medium|high|xhigh))?(?: fast)?(?: · (?:~(?:/[^\n]*)?|/[^\n]*))? · context [0-9]+% used(?: · (?:5h|weekly) [0-9]+% left)*(?: · ← for agents)?$`)
	codexAgentFooter  = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]* (?:default|none|minimal|low|medium|high|xhigh)(?: fast)? · (?:~(?:/[^·…\r\n]*)?|/[^·…\r\n]*) · ← for agents$`)
	codexReadyFooter  = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]*(?: (?:default|none|minimal|low|medium|high|xhigh))?(?: fast)? · (?:~(?:/[^·…\r\n]*)?|/[^·…\r\n]*)$`)
	codexWarning      = regexp.MustCompile(` {2,}⚠ (?:1 warning|(?:[2-9]|[1-9][0-9]+) warnings) · f2 to view$`)
	codexTitleSpinner = regexp.MustCompile(` · [⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]$`)
	screenSGR         = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	claudeFooter      = regexp.MustCompile(`^(?:⏸ (?:manual|plan) mode|⏵⏵ (?:auto mode|accept edits|bypass permissions|don't ask)) on \(shift\+tab to cycle\)(?: · esc to interrupt)?(?: · ← (?:(?:[0-9]{1,2}|99\+) agents?|for agents))?$`)
	confirmFooter     = regexp.MustCompile(`^(?:enter to (?:select|confirm)|press enter to confirm)(?: · | or )esc to cancel$`)
)

type detection struct {
	state    string
	composer string // empty, draft, or unknown; never inventory metadata.
}

// detect interprets only current provider chrome in the last eight joined lines.
// Visible capture preserves SGR so measured placeholder styling remains evidence.
func detect(provider agentruntime.Provider, text string) detection {
	unknown := detection{state: "unknown", composer: "unknown"}
	if provider != agentruntime.ProviderCodex && provider != agentruntime.ProviderClaude || len(text) > 8192 || !utf8.ValidString(text) {
		return unknown
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for len(lines) > 0 && strings.TrimSpace(screenSGR.ReplaceAllString(lines[len(lines)-1], "")) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return unknown
	}
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	result := unknown
	choice := false
	trustSelected, trustCancel, trustAllow := false, false, false
	promptIndex, workingIndex := -1, -1
	prompt := "›"
	if provider == agentruntime.ProviderClaude {
		prompt = "❯"
	}
	for i, line := range lines {
		raw := line
		line = strings.TrimSpace(screenSGR.ReplaceAllString(line, ""))
		if strings.ContainsRune(line, '\x1b') {
			return unknown
		}
		lines[i] = line
		lower := strings.ToLower(line)
		choice = choice || selectedChoice.MatchString(line)
		if workingChrome.MatchString(lower) {
			if workingIndex >= 0 {
				return unknown
			}
			workingIndex = i
		}
		if line == prompt || strings.HasPrefix(line, prompt+" ") {
			if promptIndex >= 0 {
				return unknown
			}
			promptIndex = i
			result.composer = composerState(raw, prompt)
		}
		if provider == agentruntime.ProviderClaude {
			trustSelected = trustSelected || lower == "❯ no, exit" || lower == "❯ yes, i trust this folder" || lower == "❯ yes, i accept"
			trustCancel = trustCancel || lower == "no, exit" || lower == "❯ no, exit"
			trustAllow = trustAllow || lower == "yes, i trust this folder" || lower == "yes, i accept" || lower == "❯ yes, i trust this folder" || lower == "❯ yes, i accept"
		}
	}
	last := strings.ToLower(lines[len(lines)-1])
	if provider == agentruntime.ProviderCodex {
		// Stock Codex places a complete warning notice to the right of its
		// passive footer, separated by at least two cells. It is not a dialog.
		last = strings.TrimSpace(codexWarning.ReplaceAllString(last, ""))
	}
	footer := idleFooter.MatchString(last)
	statusIndex := -1
	workingFooter := false
	if provider == agentruntime.ProviderCodex {
		// Fullscreen Codex separates its persistent status row from the
		// instructional footer. Only the recognized row directly above the
		// shortcut footer belongs to this layout.
		if footer && len(lines) > 1 {
			status := strings.ToLower(lines[len(lines)-2])
			// An unnamed thread's title-generation spinner is context metadata,
			// not evidence of an agent turn. Its source-defined suffix is optional.
			status = codexTitleSpinner.ReplaceAllString(status, "")
			if codexFooter.MatchString(status) || codexAgentFooter.MatchString(status) || codexReadyFooter.MatchString(status) {
				statusIndex = len(lines) - 2
			}
		}
		footer = footer || codexFooter.MatchString(last) || codexAgentFooter.MatchString(last) || codexReadyFooter.MatchString(last)
	} else {
		footer = footer || claudeFooter.MatchString(last)
		workingFooter = claudeFooter.MatchString(last) && strings.Contains(last, " · esc to interrupt")
	}
	if confirmFooter.MatchString(last) && (choice || trustSelected && trustCancel && trustAllow) {
		return detection{state: "blocked", composer: "unknown"}
	}
	if promptIndex < 0 {
		if workingIndex == len(lines)-1 {
			return detection{state: "working", composer: "unknown"}
		}
		return unknown
	}
	if choice || trustSelected || workingIndex >= promptIndex {
		return unknown
	}
	if workingIndex >= 0 {
		// Current work chrome immediately precedes the composer, separated only
		// by whitespace or its border. A historical spinner above output is
		// ambiguous, even when the current prompt and footer are recognizable.
		previous := promptIndex - 1
		for previous >= 0 && (lines[previous] == "" || strings.Trim(lines[previous], "─━-") == "") {
			previous--
		}
		if previous != workingIndex {
			return unknown
		}
		result.state = "working"
	}
	for i := promptIndex + 1; i < len(lines)-1; i++ {
		line := lines[i]
		if i != statusIndex && line != "" && strings.Trim(line, "─━-") != "" && result.composer != "draft" {
			return unknown
		}
	}
	if !footer {
		result.composer = "unknown"
		return result
	}
	if result.state != "working" {
		result.state = "idle"
	}
	if workingFooter {
		result.state = "working"
	}
	return result
}

// Stock Codex's placeholder is dim; typed input is not. This is presentation
// evidence, never a whitelist of placeholder words. Missing styling means draft.
func composerState(line, prompt string) string {
	dim, afterPrompt := false, false
	for len(line) > 0 {
		if strings.HasPrefix(line, "\x1b[") {
			sequence := screenSGR.FindString(line)
			if sequence == "" || !strings.HasPrefix(line, sequence) {
				return "unknown"
			}
			params := strings.Split(sequence[2:len(sequence)-1], ";")
			for i := 0; i < len(params); i++ {
				value, _ := strconv.Atoi(params[i])
				switch value {
				case 0, 22:
					dim = false
				case 2:
					dim = true
				case 38, 48, 58:
					if i+1 < len(params) {
						switch params[i+1] {
						case "5":
							i += 2
						case "2":
							i += 4
						}
					}
				}
			}
			line = line[len(sequence):]
			continue
		}
		rune, size := utf8.DecodeRuneInString(line)
		line = line[size:]
		if string(rune) == prompt {
			afterPrompt = true
			continue
		}
		if afterPrompt && rune != ' ' && rune != '\t' && !dim {
			return "draft"
		}
	}
	return "empty"
}
