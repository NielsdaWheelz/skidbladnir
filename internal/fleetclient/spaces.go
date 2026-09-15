package fleetclient

import (
	"slices"

	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

type Row struct {
	Label, Machine string
	Session        Session
	Available      bool
}

type Group struct {
	Space space.Label
	Rows  []Row
}

// Groups preserves source order within groups, including retained unavailable rows.
func Groups(peers []Peer, filter space.Filter) []Group {
	groups := []Group{}
	indices := map[space.Label]int{}
	for _, peer := range peers {
		for _, session := range peer.Sessions {
			if !filter.Matches(session.Space) {
				continue
			}
			index, present := indices[session.Space]
			if !present {
				index = len(groups)
				indices[session.Space] = index
				groups = append(groups, Group{Space: session.Space})
			}
			groups[index].Rows = append(groups[index].Rows, Row{peer.Label, peer.Machine, session, peer.OK})
		}
	}
	slices.SortFunc(groups, func(a, b Group) int { return space.Compare(a.Space, b.Space) })
	return groups
}

func ObservedSpaces(peers []Peer) []space.Label {
	labels := []space.Label{}
	for _, group := range Groups(peers, space.Filter{}) {
		if !group.Space.IsUnassigned() {
			labels = append(labels, group.Space)
		}
	}
	return labels
}

func SpaceHeading(label space.Label) string {
	if label.IsUnassigned() {
		return "unassigned"
	}
	return "space: " + label.String()
}

func SpaceFilterHeading(filter space.Filter) string {
	if filter.Kind() == space.FilterAll {
		return "all spaces"
	}
	return SpaceHeading(filter.Label())
}
