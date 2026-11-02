package tmux

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

var ErrDispatchUnknown = errors.New("tmux queue completion is unknown")

const appliedMarker = "SKIDBLADNIR_APPLIED_V1"

// Every effect returns one of:
//
//	(true, nil)  tmux completed it: the applied marker follows the last effect
//	             command in the same non-interleaved queue;
//	(false, nil) nothing ran: a guard conjunct failed, or no server runs (the
//	             client could not connect);
//	a raw start error: nothing ran;
//	any later failure wraps ErrDispatchUnknown; unexpected output is
//	errors.Join(ErrDispatchUnknown, *ProtocolError).
//
// The guard is the only authority: it pins the server lifetime and session id
// in the same queue as the effect, and an absent id refuses
// (sessionIDCondition). if-shell -F inserts its chosen branch right after
// itself, and tmux drains one client's queue before other event work, so no
// other client can act between the guard and the effect.
func (client Client) guarded(ctx context.Context, id string, server ServerIdentity, conditions []string, action string) (bool, error) {
	var stdout, stderr bytes.Buffer
	command := client.commandWithStderr(ctx, &stdout, &stderr, "-N", "if-shell", "-F", "-t", id,
		andFormatConditions(append(sessionLifetimeConditions(id, server), conditions...)),
		action+" ; "+commandText("display-message", "-p", "-l", appliedMarker),
		commandText("display-message", "-p", "-l", identityMismatchMarker))
	if err := command.Start(); err != nil {
		return false, err
	}
	if err := command.Wait(); err != nil {
		if missingServer(err, stderr.String()) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %w", ErrDispatchUnknown, err)
	}
	switch strings.TrimSuffix(stdout.String(), "\n") {
	case appliedMarker:
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.Join(ErrDispatchUnknown, &ProtocolError{Record: "guard_completion", Reason: "unexpected_marker"})
	}
}

// kill-session runs before the applied marker in the same queue, so the marker
// proves the kill completed.
func (client Client) KillSessionIfIdentity(ctx context.Context, id string, server ServerIdentity) (bool, error) {
	return client.guarded(ctx, id, server, nil, commandText("kill-session", "-t", id))
}

func (client Client) SetSessionGroupIfIdentity(ctx context.Context, id string, label group.Label, server ServerIdentity) (bool, error) {
	action := commandText("set-option", "-u", "-t", id, "--", GroupOption)
	if !label.IsUnassigned() {
		action = commandText("set-option", "-t", id, "--", GroupOption, base64.RawURLEncoding.EncodeToString([]byte(label.String())))
	}
	return client.guarded(ctx, id, server, nil, action)
}

// expected is the sampled row.Format.Character. There is no -o: -o fails on
// any existing local value, so it could never repair one.
func (client Client) AssignCharacterIfUnchanged(ctx context.Context, id, expected, character string, server ServerIdentity) (bool, error) {
	return client.guarded(ctx, id, server, []string{"#{==:#{@skid_character}," + formatLiteral(expected) + "}"},
		commandText("set-option", "-t", id, "--", "@skid_character", character))
}

type NamingGuard struct {
	ID, Name, Marker, PaneID, Title string // Marker: sampled row.Format.AutoName; PaneID/Title only for automatic renames
	Server                          ServerIdentity
}

// SetNamingIfUnchanged renames to name when name != guard.Name, and only while
// no other session holds name. Then it sets marker, or unsets it when empty, in
// one guarded queue. name holds no '#', which rename-session would expand as a
// format: sessions passes only validated manual or titleName-derived names.
func (client Client) SetNamingIfUnchanged(ctx context.Context, guard NamingGuard, name, marker string) (bool, error) {
	conditions := []string{
		"#{==:#{session_name}," + formatLiteral(guard.Name) + "}",
		"#{==:#{" + AutoNameOption + "}," + formatLiteral(guard.Marker) + "}",
	}
	if guard.PaneID != "" {
		conditions = append(conditions, "#{==:#{pane_id},"+formatLiteral(guard.PaneID)+"}", "#{==:#{pane_title},"+formatLiteral(guard.Title)+"}")
	}
	action := commandText("set-option", "-u", "-t", guard.ID, "--", AutoNameOption)
	if marker != "" {
		action = commandText("set-option", "-t", guard.ID, "--", AutoNameOption, marker)
	}
	if name != guard.Name {
		// #{S:} loops over every session, so the free-name conjunct is empty only
		// while no session holds name.
		conditions = append(conditions, "#{==:#{S:#{?#{==:#{session_name},"+formatLiteral(name)+"},1,}},}")
		action = commandText("rename-session", "-t", guard.ID, name) + " ; " + action
	}
	return client.guarded(ctx, guard.ID, guard.Server, conditions, action)
}
