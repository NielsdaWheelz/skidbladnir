package sessions

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func encodeAutoName(name string) string { return base64.RawURLEncoding.EncodeToString([]byte(name)) }
func effectiveNameMode(name, marker string) NameMode {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(marker)
	if err == nil && marker != "" && string(decoded) == name && encodeAutoName(string(decoded)) == marker {
		return NameAutomatic
	}
	return NameManual
}
func titleName(title string) string {
	if len(title) > 4096 || !utf8.ValidString(title) {
		return ""
	}
	var result strings.Builder
	separator := false
	for _, r := range title {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) || r == '\u2028' || r == '\u2029' {
			return ""
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	value := result.String()
	if len(value) > 64 {
		value = value[:64]
	}
	return strings.TrimRight(value, "-")
}
func suffixName(base, suffix string) string {
	if len(base)+len(suffix) > 64 {
		base = base[:64-len(suffix)]
	}
	return strings.TrimRight(base, "-") + suffix
}
func automaticName(base, id, current string, names map[string]struct{}) string {
	if current == base {
		return current
	}
	suffix := "-s" + strings.TrimPrefix(id, "$")
	first := suffixName(base, suffix)
	if current == first {
		return current
	}
	if at := strings.LastIndex(current, suffix+"-"); at >= 0 {
		tail := current[at+len(suffix)+1:]
		canonical := tail != "" && tail[0] != '0' && tail != "1"
		for _, digit := range tail {
			if digit < '0' || digit > '9' {
				canonical = false
				break
			}
		}
		if canonical && len(suffix)+1+len(tail) < 64 && current == suffixName(base, suffix+"-"+tail) {
			return current
		}
	}
	if _, occupied := names[base]; !occupied {
		return base
	}
	if _, occupied := names[first]; !occupied {
		return first
	}
	for n := 2; ; n++ {
		candidate := suffixName(base, suffix+"-"+strconv.Itoa(n))
		if _, occupied := names[candidate]; !occupied {
			return candidate
		}
	}
}
func (manager *Manager) normalizeNames(ctx context.Context, visible []scannedSession, names map[string]struct{}, server tmuxclient.ServerIdentity) ([]scannedSession, error) {
	result := make([]scannedSession, 0, len(visible))
	for _, session := range visible {
		guard := tmuxclient.NamingGuard{ID: session.id, Name: session.tmuxName, Marker: session.autoMarker, Server: server}
		desired, marker := "", session.autoMarker
		if effectiveNameMode(session.tmuxName, session.autoMarker) == NameAutomatic {
			pane, err := manager.tmux.Output(ctx, "read-naming-pane", "display-message", "-p", "-t", session.id, "#{pane_id}")
			if err == nil && paneIDPattern.MatchString(pane) {
				title, titleErr := manager.tmux.Output(ctx, "read-naming-title", "display-message", "-p", "-t", pane, "#{pane_title}")
				if titleErr == nil {
					if base := titleName(title); base != "" {
						desired = automaticName(base, session.id, session.tmuxName, names)
						guard.PaneID, guard.Title = pane, title
						marker = encodeAutoName(desired)
					}
				}
			}
		} else if session.autoMarker != "" {
			decoded, err := base64.RawURLEncoding.Strict().DecodeString(session.autoMarker)
			if err == nil && encodeAutoName(string(decoded)) == session.autoMarker && string(decoded) != session.tmuxName {
				marker = ""
			}
		}
		if desired != "" && desired != session.tmuxName || marker != session.autoMarker {
			_, err := manager.tmux.SetNamingIfUnchanged(ctx, guard, desired, marker)
			if err != nil { // An external destination race defers to the next inventory.
				token, tokenErr := makeIdentityToken(server, session.id)
				if tokenErr != nil {
					return nil, tokenErr
				}
				if _, _, identityErr := manager.sessionLifetimeIdentity(ctx, session.id, token); identityErr != nil {
					var missing *Error
					if !errors.As(identityErr, &missing) || missing.Code != ErrorSessionNotFound {
						return nil, identityErr
					}
				}
			}
		}
		actual, found, err := manager.scanSession(ctx, session.id)
		if err != nil {
			return nil, err
		}
		if found {
			delete(names, session.tmuxName)
			names[actual.tmuxName] = struct{}{}
			result = append(result, actual)
		}
	}
	return result, nil
}
func (manager *Manager) Rename(ctx context.Context, input RenameInput) error {
	manager.mutations.Lock()
	defer manager.mutations.Unlock()
	if input.Naming.Mode == NameManual {
		if err := validateTmuxName(input.Naming.Name); err != nil {
			return err
		}
	}
	server, name, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	marker, err := manager.sessionOption(ctx, input.TmuxID, tmuxclient.AutoNameOption)
	if err != nil {
		return err
	}
	mode := effectiveNameMode(name, marker)
	if mode != input.ExpectedNaming.Mode || mode == NameManual && name != input.ExpectedNaming.Name {
		return newSessionError(ErrorSessionNameChanged, "the session name changed. review and save again.")
	}
	desired, newMarker := input.Naming.Name, ""
	if input.Naming.Mode == NameAutomatic {
		desired = name
		newMarker = encodeAutoName(name)
	}

	if desired != name {
		scan, err := manager.scanSessions(ctx)
		if err != nil {
			return err
		}
		if _, occupied := scan.names[desired]; occupied {
			return newSessionError(ErrorSessionNameConflict, "another session on this machine uses that name.")
		}
	}
	accepted, err := manager.tmux.SetNamingIfUnchanged(ctx, tmuxclient.NamingGuard{ID: input.TmuxID, Name: name, Marker: marker, Server: server}, desired, newMarker)
	if err != nil {
		if _, actualName, lifetimeErr := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken); lifetimeErr != nil {
			return lifetimeErr
		} else if actualName == name {
			actualMarker, readErr := manager.sessionOption(ctx, input.TmuxID, tmuxclient.AutoNameOption)
			if readErr == nil && actualMarker == marker && desired != name {
				scan, scanErr := manager.scanSessions(ctx)
				if scanErr == nil {
					if _, occupied := scan.names[desired]; occupied {
						return newSessionError(ErrorSessionNameConflict, "another session on this machine uses that name.")
					}
				}
			}
		}
		return fmt.Errorf("tmux naming completion unknown: %w", err)
	}
	if !accepted {
		if _, _, err := manager.sessionLifetimeIdentity(ctx, input.TmuxID, input.IdentityToken); err != nil {
			return err
		}
		return newSessionError(ErrorSessionNameChanged, "the session name changed. review and save again.")
	}
	manager.checkpointAfterMutation(ctx, server)
	return nil
}
