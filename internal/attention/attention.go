// Package attention is the hub's notification contract: the state clients
// reconcile against and the acknowledgement they return.
// SingleLine, StateDirectory and ErrUnavailable also serve callers outside
// notifications until they move (docs/issues/attention-shared-helpers.md).
package attention

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var ErrUnavailable = errors.New("notifications unavailable")

type Availability string

const (
	Fresh Availability = "fresh"
	Gap   Availability = "gap" // the hub could not read this machine: keep its notices
)

// Delivery is the hub's verdict for one session's notice.
type Delivery string

const (
	Post  Delivery = "post"  // current and unhandled: show it, alerting once per episode
	Keep  Delivery = "keep"  // unhandled but unverified: never post, never remove
	Clear Delivery = "clear" // resolved, handled, replaced or no cause: remove any copy
)

// State is GET /v1/notifications/state.
type State struct {
	Machines []Machine `json:"machines"`
}

type Machine struct {
	Machine      string       `json:"machine"`
	Availability Availability `json:"availability"`
	Sessions     []Session    `json:"sessions,omitempty"` // fresh machines only
}

// Session is one session lifetime. Title, Body and Episode are set unless
// Delivery is Clear; ReadyGeneration and Foreground only while ready is pending.
type Session struct {
	Slot            string      `json:"slot"` // native notice identifier
	Ref             string      `json:"ref"`  // exact click and entry target
	Title           string      `json:"title,omitempty"`
	Body            string      `json:"body,omitempty"`
	Episode         int64       `json:"episode,omitempty"`
	Delivery        Delivery    `json:"delivery"`
	ReadyGeneration int64       `json:"readyGeneration,omitempty"`
	Foreground      *Foreground `json:"foreground,omitempty"`
}

type Foreground struct {
	Provider      string `json:"provider"`
	PID           int64  `json:"pid"`
	StartIdentity string `json:"startIdentity"`
}

func SameForeground(left, right *Foreground) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// Handled is POST /v1/notifications/handled. Each nonzero field applies only
// while it equals that session's current value; anything else is a no-op.
type Handled struct {
	Slot            string `json:"slot"`
	Episode         int64  `json:"episode"`
	ReadyGeneration int64  `json:"readyGeneration"`
}

func SingleLine(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, text)
}

func StateDirectory() (string, error) {
	directory := os.Getenv("XDG_STATE_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ErrUnavailable
		}
		directory = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(directory) {
		return "", ErrUnavailable
	}
	return filepath.Join(directory, "skidbladnir"), nil
}
