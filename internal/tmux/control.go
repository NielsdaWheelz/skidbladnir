package tmux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInputInvalid = errors.New("invalid agent terminal input")

type Capture struct {
	Text      string
	Alternate bool
	Truncated bool
}

// CapturePane reads tmux's retained tail, not the client's scrolled viewport.
func (client Client) CapturePane(ctx context.Context, pane string, maxBytes int) (Capture, error) {
	if !validPane(pane) || maxBytes < 1 || maxBytes > 32768 {
		return Capture{}, errors.New("invalid pane capture")
	}
	alternate, err := client.Output(ctx, "capture-metadata", "display-message", "-p", "-t", pane, "#{alternate_on}")
	if err != nil {
		return Capture{}, err
	}
	if alternate != "0" && alternate != "1" {
		return Capture{}, errors.New("invalid pane capture metadata")
	}
	// A byte bound is applied after capture; limiting lines also bounds work for
	// a large configured scrollback. Joined wrapped lines preserve readable text.
	output := captureTail{limit: maxBytes}
	command := client.command(ctx, nil, "capture-pane", "-p", "-J", "-t", pane, "-S", "-"+strconv.Itoa(maxBytes))
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return Capture{}, errors.New("pane capture failed")
	}
	text := strings.TrimSuffix(string(output.bytes), "\n")
	for len(text) > 0 && !utf8.RuneStart(text[0]) {
		text = text[1:]
	}
	truncated := output.truncated
	return Capture{Text: text, Alternate: alternate == "1", Truncated: truncated}, nil
}

func (client Client) Paste(ctx context.Context, pane, text string) error {
	if !validPane(pane) || text == "" || len(text) > 32768 || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return ErrInputInvalid
	}
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return err
	}
	name := "skid-agent-" + hex.EncodeToString(entropy[:])
	// Cleanup is registered before loading: a failed acknowledgement can follow
	// a successful load. Only this operation's unique transient buffer is addressed.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Run(cleanup, "clear-agent-input", "delete-buffer", "-b", name) // justify-ignore-error: paste may already have deleted this operation's buffer.
	}()
	load := client.command(ctx, nil, "load-buffer", "-b", name, "-")
	load.Stdin = strings.NewReader(text)
	if err := load.Run(); err != nil {
		return errors.New("load agent input failed")
	}
	err := client.Run(ctx, "paste-agent-input", "paste-buffer", "-p", "-r", "-d", "-b", name, "-t", pane, ";", "send-keys", "-t", pane, "Enter")
	return err
}

func (client Client) Keys(ctx context.Context, pane string, keys []string) error {
	if !validPane(pane) || len(keys) < 1 || len(keys) > 16 {
		return ErrInputInvalid
	}
	args := []string{"-t", pane}
	for _, key := range keys {
		var encoded string
		switch key {
		case "enter":
			encoded = "Enter"
		case "escape":
			encoded = "Escape"
		case "ctrl-c":
			encoded = "C-c"
		case "up":
			encoded = "Up"
		case "down":
			encoded = "Down"
		case "left":
			encoded = "Left"
		case "right":
			encoded = "Right"
		case "tab":
			encoded = "Tab"
		case "backspace":
			encoded = "BSpace"
		case "page-up":
			encoded = "PPage"
		case "page-down":
			encoded = "NPage"
		default:
			return ErrInputInvalid
		}
		args = append(args, encoded)
	}
	return client.Run(ctx, "agent-keys", "send-keys", args...)
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
