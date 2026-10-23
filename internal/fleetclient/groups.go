package fleetclient

import (
	"cmp"
	"slices"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// Row is one session with its peer's facts: where it is listed, when that
// inventory observed it, and whether the peer is currently available.
type Row struct {
	Label, Machine, ObservedAt string
	Session                    Session
	Available                  bool
}

func (peer Peer) Row(session Session) Row {
	return Row{peer.Label, peer.Machine, peer.ObservedAt, session, peer.OK}
}

type Group struct {
	Label group.Label
	Rows  []Row
}

// Groups preserves machine order and orders sessions by numeric tmux id.
func Groups(peers []Peer, filter group.Filter) []Group {
	groups := []Group{}
	indices := map[group.Label]int{}
	for _, peer := range peers {
		sessions := slices.Clone(peer.Sessions)
		slices.SortStableFunc(sessions, CompareSessions)
		for _, session := range sessions {
			if !filter.Matches(session.Group) {
				continue
			}
			index, present := indices[session.Group]
			if !present {
				index = len(groups)
				indices[session.Group] = index
				groups = append(groups, Group{Label: session.Group})
			}
			groups[index].Rows = append(groups[index].Rows, peer.Row(session))
		}
	}
	slices.SortFunc(groups, func(a, b Group) int { return group.Compare(a.Label, b.Label) })
	return groups
}

func ObservedGroups(peers []Peer) []group.Label {
	labels := []group.Label{}
	for _, group := range Groups(peers, group.Filter{}) {
		if !group.Label.IsUnassigned() {
			labels = append(labels, group.Label)
		}
	}
	return labels
}

// CompareSessions orders sessions by numeric tmux id.
func CompareSessions(a, b Session) int {
	l := strings.TrimLeft(strings.TrimPrefix(a.Ref.TmuxID, "$"), "0")
	r := strings.TrimLeft(strings.TrimPrefix(b.Ref.TmuxID, "$"), "0")
	if order := cmp.Compare(len(l), len(r)); order != 0 {
		return order
	}
	return strings.Compare(l, r)
}
