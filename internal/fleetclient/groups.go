package fleetclient

import (
	"cmp"
	"slices"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

type Row struct {
	Label, Machine string
	Session        Session
	Available      bool
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
		slices.SortStableFunc(sessions, compareSessions)
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
			groups[index].Rows = append(groups[index].Rows, Row{peer.Label, peer.Machine, session, peer.OK})
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

func GroupHeading(label group.Label) string {
	if label.IsUnassigned() {
		return "unassigned"
	}
	return label.String()
}

func compareSessions(a, b Session) int {
	left, _ := DecodeReference(a.Ref)
	right, _ := DecodeReference(b.Ref)
	l := strings.TrimLeft(strings.TrimPrefix(left.TmuxID, "$"), "0")
	r := strings.TrimLeft(strings.TrimPrefix(right.TmuxID, "$"), "0")
	if order := cmp.Compare(len(l), len(r)); order != 0 {
		return order
	}
	return strings.Compare(l, r)
}
