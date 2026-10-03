package sessionui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
)

// groupChoice distinguishes literal acceptance from an observed suggestion.
// The zero label is the explicit unassigned action, never a named suggestion.
type groupChoice struct {
	label   group.Label
	literal bool
}

// groupChoices derives typing assistance from the current draft and inventory.
// Stored equality stays exact; only matching folds ASCII letters.
func groupChoices(draft string, observed []group.Label) []groupChoice {
	canonical, err := group.ParseDraft(draft)
	query := draft
	if err == nil {
		query = canonical.String()
	}
	fold := func(text string) string {
		bytes := []byte(text)
		for index, value := range bytes {
			if value >= 'A' && value <= 'Z' {
				bytes[index] += 'a' - 'A'
			}
		}
		return string(bytes)
	}
	folded := fold(query)
	type match struct {
		label group.Label
		rank  int
	}
	matches := []match{}
	exact := false
	for _, label := range observed {
		text := label.String()
		value := fold(text)
		rank := -1
		switch {
		case text == query:
			rank, exact = 0, true
		case value == folded:
			rank = 1
		case strings.HasPrefix(value, folded):
			rank = 2
		case strings.Contains(value, folded):
			rank = 3
		default:
			remaining := []rune(folded)
			for _, scalar := range value {
				if len(remaining) > 0 && scalar == remaining[0] {
					remaining = remaining[1:]
				}
			}
			if len(remaining) == 0 {
				rank = 4
			}
		}
		if rank >= 0 {
			matches = append(matches, match{label, rank})
		}
	}
	// ObservedGroups already owns label order; rank ties preserve it.
	slices.SortStableFunc(matches, func(a, b match) int { return cmp.Compare(a.rank, b.rank) })
	choices := make([]groupChoice, 0, len(matches)+2)
	for _, match := range matches {
		choices = append(choices, groupChoice{label: match.label})
	}
	if err == nil && !canonical.IsUnassigned() && !exact {
		choices = append(choices, groupChoice{label: canonical, literal: true})
	}
	return append(choices, groupChoice{})
}

func (m *model) groupChoices(draft string) []groupChoice {
	return groupChoices(draft, fleetclient.ObservedGroups(m.scopedPeers()))
}

// groupSelection stores only a highlight's identity. Drafts and choices stay
// with their existing owners, so observation cannot replace a user's text.
type groupSelection struct{ fieldSelection[groupChoice] }

func (selection *groupSelection) reset(draft string, choices []groupChoice) {
	selection.choice = nil
	if draft == "" {
		selection.choice = &choices[len(choices)-1]
	} else if !choices[0].label.IsUnassigned() {
		selection.choice = &choices[0]
	}
}

// Prefills and returning focus preserve the accepted literal value, even when
// a different observed label would be the best typing suggestion.
func (selection *groupSelection) prefill(draft string, choices []groupChoice) {
	selection.choice = nil
	label, err := group.ParseDraft(draft)
	if err != nil {
		return
	}
	for _, choice := range choices {
		if choice.label == label {
			selection.choice = &choice
			return
		}
	}
}

func (selection *groupSelection) accept(draft string, choices []groupChoice) (string, error) {
	if choice, _, selected := selection.current(choices); selected {
		return choice.label.String(), nil
	}
	label, err := group.ParseDraft(draft)
	return label.String(), err
}
