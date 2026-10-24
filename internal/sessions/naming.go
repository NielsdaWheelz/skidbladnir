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

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

func encodeAutoName(name string) string { return base64.RawURLEncoding.EncodeToString([]byte(name)) }
func effectiveNameMode(name, marker string) api.NameMode {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(marker)
	if err == nil && marker != "" && string(decoded) == name && encodeAutoName(string(decoded)) == marker {
		return api.NameAutomatic
	}
	return api.NameManual
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

// normalizeNames applies automatic naming (docs/architecture.md, naming) to
// rows, normalizeCharacters' copy, in place. A write that tmux applied is
// projected; a refused or uncertain write keeps the observed row, and the next
// inventory observes the outcome. Guards compare format lookup, so the marker
// they expect is the row's format value; mode comes from the exact local value.
func (manager *Manager) normalizeNames(ctx context.Context, rows []tmuxclient.InventoryRow, names map[string]struct{}, server tmuxclient.ServerIdentity) ([]tmuxclient.InventoryRow, error) {
	for i, row := range rows {
		guard := tmuxclient.NamingGuard{ID: row.ID, Name: row.Name, Marker: row.Format.AutoName, Server: server}
		desired, marker := row.Name, row.LocalSession.AutoName
		if effectiveNameMode(row.Name, row.LocalSession.AutoName) == api.NameAutomatic {
			if base := titleName(row.Title); base != "" {
				desired = automaticName(base, row.ID, row.Name, names)
				guard.PaneID, guard.Title = row.PaneID, row.Title
				marker = encodeAutoName(desired)
			}
		} else if row.LocalSession.AutoName != "" {
			decoded, err := base64.RawURLEncoding.Strict().DecodeString(row.LocalSession.AutoName)
			if err == nil && encodeAutoName(string(decoded)) == row.LocalSession.AutoName && string(decoded) != row.Name {
				marker = ""
			}
		}
		if desired == row.Name && marker == row.LocalSession.AutoName {
			continue
		}
		applied, err := manager.tmux.SetNamingIfUnchanged(ctx, guard, desired, marker)
		if errors.Is(err, tmuxclient.WorkspaceProtocolInvalid) {
			return nil, err
		}
		if err != nil || !applied {
			continue
		}
		delete(names, row.Name)
		names[desired] = struct{}{}
		rows[i].Name, rows[i].LocalSession.AutoName = desired, marker
	}
	return rows, nil
}

// Rename reads its session once, writes in one guarded queue, and classifies a
// refusal by one more read. The guard also requires the new name to be free, so
// a refusal whose other observed conjuncts still hold is a name conflict.
func (manager *Manager) Rename(ctx context.Context, input RenameInput) error {
	if err := manager.acquire(ctx); err != nil {
		return err
	}
	locked := true
	defer func() {
		if locked {
			manager.release()
		}
	}()
	if input.Naming.Mode == api.NameManual {
		if err := validateTmuxName(input.Naming.Name); err != nil {
			return err
		}
	}
	server, row, err := manager.resolve(ctx, input.TmuxID, input.IdentityToken)
	if err != nil {
		return err
	}
	changed := newSessionError(ErrorSessionNameChanged, "the session name changed. review and save again.")
	mode := effectiveNameMode(row.Name, row.LocalSession.AutoName)
	if mode != input.ExpectedNaming.Mode || mode == api.NameManual && row.Name != input.ExpectedNaming.Name {
		return changed
	}
	desired, marker := input.Naming.Name, ""
	if input.Naming.Mode == api.NameAutomatic {
		desired, marker = row.Name, encodeAutoName(row.Name)
	}
	applied, err := manager.tmux.SetNamingIfUnchanged(ctx, tmuxclient.NamingGuard{ID: row.ID, Name: row.Name, Marker: row.Format.AutoName, Server: server}, desired, marker)
	if err != nil {
		if _, _, resolveErr := manager.resolve(ctx, input.TmuxID, input.IdentityToken); resolveErr != nil {
			return resolveErr
		}
		return fmt.Errorf("tmux naming completion unknown: %w", err)
	}
	if !applied {
		_, current, err := manager.resolve(ctx, input.TmuxID, input.IdentityToken)
		if err != nil {
			return err
		}
		if current.Name == row.Name && current.Format.AutoName == row.Format.AutoName && desired != row.Name {
			return newSessionError(ErrorSessionNameConflict, "another session on this machine uses that name.")
		}
		return changed
	}
	manager.release()
	locked = false
	manager.checkpointAfterMutation(ctx, server)
	return nil
}
