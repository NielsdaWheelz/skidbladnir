package tmux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInputInvalid  = errors.New("invalid terminal input")
	ErrTargetChanged = errors.New("terminal target changed")
	ErrUnavailable   = errors.New("terminal unavailable")
	ErrWriteUnknown  = errors.New("terminal input delivery is unknown")
)

const (
	inputWrittenMarker     = "SKIDBLADNIR_INPUT_WRITTEN"
	inputUnavailableMarker = "SKIDBLADNIR_INPUT_UNAVAILABLE"
)

type PaneTarget struct {
	SessionID string
	PaneID    string
	Server    ServerIdentity
}

type Capture struct {
	Text      string
	Alternate bool
	Truncated bool
}

func (target PaneTarget) valid() bool {
	return sessionIDPattern.MatchString(target.SessionID) && validPane(target.PaneID) && target.Server.valid()
}

func (target PaneTarget) condition() string {
	return andFormatConditions(append(sessionLifetimeConditions(target.SessionID, target.Server), "#{==:#{pane_id},"+target.PaneID+"}"))
}

// refusal is the else branch of a guard that extends target.condition(). It
// prints marker when the target still holds, so only the extension failed, and
// identityMismatchMarker when the target itself changed.
func (target PaneTarget) refusal(marker string) string {
	return "if-shell -F -t '" + target.SessionID + "' '" + target.condition() + "' 'display-message -p -l " + marker + "' 'display-message -p -l " + identityMismatchMarker + "'"
}

// CapturePane is the public plain-text read: the retained tail, or the visible
// screen of an alternate-screen program, under the exact target guard.
func (client Client) CapturePane(ctx context.Context, target PaneTarget, maxBytes int) (Capture, error) {
	if !target.valid() || maxBytes < 1 || maxBytes > 32768 {
		return Capture{}, ErrInputInvalid
	}
	capture := "capture-pane -p -J -t " + target.PaneID + " -S -" + strconv.Itoa(maxBytes)
	branch := "display-message -p '#{alternate_on}' ; if-shell -F -t '" + target.PaneID + "' '#{alternate_on}' 'capture-pane -p -J -t " + target.PaneID + "' '" + capture + "'"
	tail := captureTail{limit: maxBytes}
	output := captureOutput{body: &tail}
	command := client.command(ctx, nil, "-N", "if-shell", "-F", "-t", target.SessionID, target.condition(), branch, "display-message -p -l '"+identityMismatchMarker+"'")
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return Capture{}, ErrUnavailable
	}
	if output.header == identityMismatchMarker {
		return Capture{}, ErrTargetChanged
	}
	if output.header != "0" && output.header != "1" {
		return Capture{}, ErrUnavailable
	}
	text := strings.TrimSuffix(string(tail.bytes), "\n")
	for len(text) > 0 && !utf8.RuneStart(text[0]) {
		text = text[1:]
	}
	return Capture{Text: text, Alternate: output.header == "1", Truncated: tail.truncated}, nil
}

// Paste stages a unique buffer before the caller's final foreground check.
// Both paste and submit are inside one successful lifetime/active-pane branch.
func (client Client) Paste(ctx context.Context, target PaneTarget, text string, revalidate func() error) error {
	if !target.valid() || text == "" || len(text) > 32768 || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return ErrInputInvalid
	}
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return ErrUnavailable
	}
	name := "skid-input-" + hex.EncodeToString(entropy[:])
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = client.Run(cleanup, "clear-terminal-input", "-N", "delete-buffer", "-b", name) // justify-ignore-error: bounded cleanup only removes this operation's transient buffer; tmux may already be unavailable.
	}()
	load := client.command(ctx, nil, "-N", "load-buffer", "-b", name, "-")
	load.Stdin = strings.NewReader(text)
	if err := load.Run(); err != nil {
		return ErrUnavailable
	}
	if err := revalidate(); err != nil {
		return err
	}
	branch := "paste-buffer -p -r -d -b '" + name + "' -t '" + target.PaneID + "' ; send-keys -t '" + target.PaneID + "' Enter"
	return client.write(ctx, target, branch)
}

func (client Client) Keys(ctx context.Context, target PaneTarget, keys []string) error {
	if !target.valid() || len(keys) < 1 || len(keys) > 16 {
		return ErrInputInvalid
	}
	encoded := make([]string, 0, len(keys))
	for _, key := range keys {
		switch key {
		case "enter":
			encoded = append(encoded, "Enter")
		case "escape":
			encoded = append(encoded, "Escape")
		case "ctrl-c":
			encoded = append(encoded, "C-c")
		case "up":
			encoded = append(encoded, "Up")
		case "down":
			encoded = append(encoded, "Down")
		case "left":
			encoded = append(encoded, "Left")
		case "right":
			encoded = append(encoded, "Right")
		case "tab":
			encoded = append(encoded, "Tab")
		case "backspace":
			encoded = append(encoded, "BSpace")
		case "page-up":
			encoded = append(encoded, "PPage")
		case "page-down":
			encoded = append(encoded, "NPage")
		default:
			return ErrInputInvalid
		}
	}
	return client.write(ctx, target, "send-keys -t '"+target.PaneID+"' "+strings.Join(encoded, " "))
}

func (client Client) write(ctx context.Context, target PaneTarget, branch string) error {
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	condition := andFormatConditions([]string{target.condition(), "#{==:#{pane_dead},0}"})
	// A dead pane is positively unavailable; a different active pane/lifetime is stale.
	var output strings.Builder
	command := client.command(ctx, nil, "-N", "if-shell", "-F", "-t", target.SessionID, condition, branch+" ; display-message -p -l '"+inputWrittenMarker+"'",
		target.refusal(inputUnavailableMarker))
	command.Stdout = &output
	if err := command.Start(); err != nil {
		return ErrUnavailable
	}
	if err := command.Wait(); err != nil {
		return ErrWriteUnknown
	}
	switch strings.TrimSuffix(output.String(), "\n") {
	case inputWrittenMarker:
		return nil
	case identityMismatchMarker:
		return ErrTargetChanged
	case inputUnavailableMarker:
		return ErrUnavailable
	default:
		return ErrWriteUnknown
	}
}

func validPane(value string) bool {
	if !strings.HasPrefix(value, "%") || len(value) < 2 {
		return false
	}
	for _, c := range value[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// captureOutput keeps the small protocol header separate from the bounded body.
type captureOutput struct {
	header     string
	headerDone bool
	body       io.Writer
}

func (output *captureOutput) Write(contents []byte) (int, error) {
	count := len(contents)
	if !output.headerDone {
		header, rest, found := strings.Cut(string(contents), "\n")
		output.header += header
		if len(output.header) > 128 {
			return 0, errors.New("invalid capture header")
		}
		if !found {
			return count, nil
		}
		output.headerDone = true
		contents = []byte(rest)
	}
	_, err := output.body.Write(contents)
	return count, err
}

// captureTail bounds memory while tmux emits a potentially large retained tail.
type captureTail struct {
	bytes     []byte
	limit     int
	truncated bool
}

func (tail *captureTail) Write(contents []byte) (int, error) {
	count := len(contents)
	if len(tail.bytes)+count > tail.limit {
		tail.truncated = true
	}
	if count >= tail.limit {
		tail.bytes = append(tail.bytes[:0], contents[count-tail.limit:]...)
		return count, nil
	}
	if overflow := len(tail.bytes) + count - tail.limit; overflow > 0 {
		tail.bytes = tail.bytes[overflow:]
	}
	tail.bytes = append(tail.bytes, contents...)
	return count, nil
}
