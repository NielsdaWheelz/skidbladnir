package agentcontrol

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

var (
	workingChrome       = regexp.MustCompile(`^[•◦✻✽✶✳✢·] .+\([^\n]*esc to interrupt[^\n]*\)$`)
	claudeWorkingChrome = regexp.MustCompile(`^[✻✽✶✳✢·] [^\n]+…$`)
	selectedChoice      = regexp.MustCompile(`^[❯›] [1-9][0-9]*[.)] .+$`)
	idleFooter          = regexp.MustCompile(`^\? for shortcuts(?:\s+[0-9]+% context left)?$`)
	codexFooter         = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]*(?: (?:default|none|minimal|low|medium|high|xhigh))?(?: fast)?(?: · (?:~(?:/[^\n]*)?|/[^\n]*))? · context [0-9]+% used(?: · (?:5h|weekly) [0-9]+% left)*(?: · ← for agents)?$`)
	codexAgentFooter    = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]* (?:default|none|minimal|low|medium|high|xhigh)(?: fast)? · (?:~(?:/[^·…\r\n]*)?|/[^·…\r\n]*) · ← for agents$`)
	codexReadyFooter    = regexp.MustCompile(`^gpt-[a-z0-9][a-z0-9._-]*(?: (?:default|none|minimal|low|medium|high|xhigh))?(?: fast)? · (?:~(?:/[^·…\r\n]*)?|/[^·…\r\n]*)(?: · [⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏] +⚠ [1-9][0-9]* warnings? · f2 to view)?$`)
	codexWarning        = regexp.MustCompile(` {2,}⚠ (?:1 warning|(?:[2-9]|[1-9][0-9]+) warnings) · f2 to view$`)
	codexTitleSpinner   = regexp.MustCompile(` · [⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]$`)
	screenSGR           = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	claudeFooter        = regexp.MustCompile(`^(?:⏸ (?:manual|plan) mode|⏵⏵ (?:auto mode|accept edits|bypass permissions|don't ask)) on \(shift\+tab to cycle\)(?: · esc to interrupt)?(?: · ← (?:(?:[0-9]{1,2}|99\+) agents?|for agents))?$`)
	confirmFooter       = regexp.MustCompile(`^(?:enter to (?:select|confirm)|press enter to confirm)(?: · | or )esc to cancel$`)
)

type detection struct {
	state    string
	composer string // empty, draft, or unknown; never inventory metadata.
}

// detect interprets only current provider chrome in the last eight joined lines.
// Visible capture preserves SGR so measured placeholder styling remains evidence.
func detect(provider agentruntime.Provider, text string) detection {
	result := detection{state: "unknown", composer: "unknown"}
	if provider != agentruntime.ProviderCodex && provider != agentruntime.ProviderClaude || len(text) > 8192 || !utf8.ValidString(text) {
		return result
	}
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.TrimSpace(screenSGR.ReplaceAllString(lines[len(lines)-1], "")) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return result
	}
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	last := strings.ToLower(strings.TrimSpace(screenSGR.ReplaceAllString(lines[len(lines)-1], "")))
	if provider == agentruntime.ProviderCodex {
		// A complete right-side warning is passive footer metadata, not a dialog.
		last = strings.TrimSpace(codexWarning.ReplaceAllString(last, ""))
		last = codexTitleSpinner.ReplaceAllString(last, "")
	}
	choice, dialog := false, false
	trustSelected, trustCancel, trustAllow := false, false, false
	promptIndex, historyPromptIndex, workingIndex := -1, -1, -1
	prompt := "›"
	if provider == agentruntime.ProviderClaude {
		prompt = "❯"
	}
	for i, line := range lines {
		raw := line
		plain := screenSGR.ReplaceAllString(line, "")
		line = strings.TrimSpace(plain)
		if strings.ContainsRune(line, '\x1b') {
			return detection{state: "unknown", composer: "unknown"}
		}
		lower := strings.ToLower(line)
		choice = choice || selectedChoice.MatchString(line)
		dialog = dialog || confirmFooter.MatchString(lower)
		if workingChrome.MatchString(lower) || provider == agentruntime.ProviderClaude && claudeFooter.MatchString(last) && strings.Contains(last, " · esc to interrupt") && claudeWorkingChrome.MatchString(line) {
			workingIndex = i
		}
		// The bottommost column-zero prompt owns the recognized footer below it.
		// Earlier history prompts and indented wrapped input do not own this composer.
		if strings.HasPrefix(plain, prompt) && (line == prompt || strings.HasPrefix(line, prompt+" ") || strings.HasPrefix(line, prompt+"\u00a0")) {
			historyPromptIndex = promptIndex
			promptIndex = i
			result.composer = composerState(raw, prompt)
		}
		if provider == agentruntime.ProviderClaude {
			trustSelected = trustSelected || lower == "❯ no, exit" || lower == "❯ yes, i trust this folder" || lower == "❯ yes, i accept"
			trustCancel = trustCancel || lower == "no, exit" || lower == "❯ no, exit"
			trustAllow = trustAllow || lower == "yes, i trust this folder" || lower == "yes, i accept" || lower == "❯ yes, i trust this folder" || lower == "❯ yes, i accept"
		}
	}
	footer, workingFooter := false, false
	statusIndex := -1
	if promptIndex >= 0 && promptIndex < len(lines)-1 {
		if provider == agentruntime.ProviderCodex {
			footer = idleFooter.MatchString(last) || codexFooter.MatchString(last) || codexAgentFooter.MatchString(last) || codexReadyFooter.MatchString(last)
			// Fullscreen status has the stock two-column footer inset, a spacing
			// row above, and the shortcut row immediately below. Do not trim away
			// input indentation or admit a footer-shaped wrapped composer line.
			candidate := len(lines) - 2
			shortcut := screenSGR.ReplaceAllString(lines[len(lines)-1], "")
			if idleFooter.MatchString(last) && strings.HasPrefix(shortcut, "  ?") && candidate > promptIndex+1 && strings.TrimSpace(screenSGR.ReplaceAllString(lines[candidate-1], "")) == "" {
				plain := strings.TrimRight(screenSGR.ReplaceAllString(lines[candidate], ""), " \t\r")
				if status, inset := strings.CutPrefix(plain, "  "); inset {
					status = codexTitleSpinner.ReplaceAllString(strings.ToLower(status), "")
					if codexFooter.MatchString(status) || codexAgentFooter.MatchString(status) || codexReadyFooter.MatchString(status) {
						statusIndex = candidate
					}
				}
			}
		} else {
			footer = claudeFooter.MatchString(last)
			workingFooter = footer && strings.Contains(last, " · esc to interrupt")
		}
	}
	if choice && (dialog || provider == agentruntime.ProviderCodex && last == "enter select · esc back") || trustSelected && trustCancel && trustAllow && confirmFooter.MatchString(last) {
		return detection{state: "blocked", composer: "unknown"}
	}
	if promptIndex >= 0 {
		border := ""
		if provider == agentruntime.ProviderClaude && promptIndex > 0 {
			above := strings.TrimRight(screenSGR.ReplaceAllString(lines[promptIndex-1], ""), " \t\r")
			if strings.HasPrefix(above, "─") && strings.Trim(above, "─") == "" {
				border = above
			}
		}
		if !workingFooter && workingIndex > historyPromptIndex && workingIndex < promptIndex {
			// Current work must touch this composer through whitespace or its
			// paired Claude border. Output and other separators make it stale.
			previous := promptIndex - 1
			if border != "" && footer && len(lines)-2 > promptIndex && strings.TrimRight(screenSGR.ReplaceAllString(lines[len(lines)-2], ""), " \t\r") == border {
				previous--
			}
			for previous >= 0 && strings.TrimSpace(screenSGR.ReplaceAllString(lines[previous], "")) == "" {
				previous--
			}
			if previous != workingIndex {
				return detection{state: "unknown", composer: "unknown"}
			}
		}
		end := len(lines)
		if footer {
			end--
		}
		for i := promptIndex + 1; i < end; i++ {
			plain := strings.TrimRight(screenSGR.ReplaceAllString(lines[i], ""), " \t\r")
			// Claude's paired column-zero borders bound the current composer.
			// An indented or additional separator belongs to input, not chrome.
			if i == statusIndex || strings.TrimSpace(plain) == "" || footer && i == end-1 && border != "" && plain == border {
				continue
			}
			// Wrapped input is a draft. Unrecognized chrome cannot prove empty.
			if result.composer != "draft" {
				result.composer = "unknown"
			}
		}
	}
	if workingFooter || workingIndex > historyPromptIndex && (promptIndex < 0 && workingIndex == len(lines)-1 || promptIndex >= 0 && workingIndex < promptIndex) {
		result.state = "working"
	} else if promptIndex >= 0 && footer && !choice && !trustSelected {
		result.state = "idle"
	}
	if !footer {
		result.composer = "unknown"
	}
	return result
}

// Stock Codex's placeholder is dim; typed input is not. This is presentation
// evidence, never a whitelist of placeholder words. Missing styling means draft.
func composerState(line, prompt string) string {
	dim, afterPrompt, draft := false, false, false
	for len(line) > 0 {
		if strings.HasPrefix(line, "\x1b[") {
			sequence := screenSGR.FindString(line)
			if sequence == "" || len(sequence) > 64 || !strings.HasPrefix(line, sequence) {
				return "unknown"
			}
			params := strings.Split(sequence[2:len(sequence)-1], ";")
			if len(params) > 16 {
				return "unknown"
			}
			if len(params) == 1 && params[0] == "" {
				params[0] = "0"
			}
			for i := 0; i < len(params); i++ {
				value, err := strconv.Atoi(params[i])
				if err != nil {
					return "unknown"
				}
				switch value {
				case 0, 22:
					dim = false
				case 2:
					dim = true
				case 1, 3, 4, 7, 23, 24, 27, 39, 49:
					// These observed attributes do not change placeholder dimness.
				case 38, 48:
					if i+1 >= len(params) {
						return "unknown"
					}
					count := 0
					switch params[i+1] {
					case "5":
						count = 1
					case "2":
						count = 3
					default:
						return "unknown"
					}
					if i+1+count >= len(params) {
						return "unknown"
					}
					for _, component := range params[i+2 : i+2+count] {
						color, err := strconv.Atoi(component)
						if err != nil || color < 0 || color > 255 {
							return "unknown"
						}
					}
					i += 1 + count
				default:
					if !(value >= 30 && value <= 37 || value >= 40 && value <= 47 || value >= 90 && value <= 97 || value >= 100 && value <= 107) {
						return "unknown"
					}
				}
			}
			line = line[len(sequence):]
			continue
		}
		rune, size := utf8.DecodeRuneInString(line)
		line = line[size:]
		if !afterPrompt && string(rune) == prompt {
			afterPrompt = true
			continue
		}
		if !afterPrompt && !unicode.IsSpace(rune) {
			return "unknown"
		}
		if afterPrompt && !unicode.IsSpace(rune) && !dim {
			draft = true
		}
	}
	if draft {
		return "draft"
	}
	return "empty"
}
