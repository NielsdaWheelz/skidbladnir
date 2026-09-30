package tmux

import (
	"context"
	"errors"
)

// NamingGuard captures facts sampled by the manager. Marker is a local option.
type NamingGuard struct {
	ID, Name, Marker, PaneID, Title string
	Server                          ServerIdentity
}

func (client Client) SetNamingIfUnchanged(ctx context.Context, guard NamingGuard, name, marker string) (bool, error) {
	if !sessionIDPattern.MatchString(guard.ID) || !guard.Server.valid() || guard.Name == "" || name != "" && name != guard.Name && !tmuxCommandTokenPattern.MatchString(name) || marker != "" && !tmuxCommandTokenPattern.MatchString(marker) {
		return false, errors.New("invalid tmux naming mutation")
	}
	conditions := append(sessionLifetimeConditions(guard.ID, guard.Server), "#{==:#{session_name},"+formatLiteral(guard.Name)+"}")
	// The reserved key has supported definitions only at session scope.
	conditions = append(conditions, "#{==:#{"+AutoNameOption+"},"+formatLiteral(guard.Marker)+"}")
	if guard.PaneID != "" {
		if !paneIDPattern.MatchString(guard.PaneID) {
			return false, errors.New("invalid naming pane")
		}
		conditions = append(conditions, "#{==:#{pane_id},"+formatLiteral(guard.PaneID)+"}", "#{==:#{pane_title},"+formatLiteral(guard.Title)+"}")
	}
	command := ""
	if name != "" && name != guard.Name {
		command = "rename-session -t '" + guard.ID + "' '" + name + "' ; "
	}
	if marker == "" {
		command += "set-option -u -t '" + guard.ID + "' -- " + AutoNameOption
	} else {
		command += "set-option -t '" + guard.ID + "' -- " + AutoNameOption + " " + marker
	}
	command += " ; display-message -p -l '" + renameSuccessMarker + "'"
	output, err := client.Output(ctx, "set-naming-if-unchanged", "if-shell", "-F", "-t", guard.ID, andFormatConditions(conditions), command, "display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		return false, err
	}
	switch output {
	case renameSuccessMarker:
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.New("unexpected naming mutation output")
	}
}
