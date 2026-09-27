package fleetclient

import (
	"slices"

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

// Groups preserves source order within groups, including retained unavailable rows.
func Groups(peers []Peer, filter group.Filter) []Group {
	groups := []Group{}
	indices := map[group.Label]int{}
	for _, peer := range peers {
		for _, session := range peer.Sessions {
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
	return "group: " + label.String()
}

func GroupFilterHeading(filter group.Filter) string {
	if filter.Kind() == group.FilterAll {
		return "all groups"
	}
	return GroupHeading(filter.Label())
}
