package sessionui

import (
	"fmt"
	"slices"
)

// fieldSelection owns a highlight's identity, never the editable draft. Choice
// values include their domain action kind, so refresh cannot change its meaning.
type fieldSelection[T comparable] struct{ choice *T }

func (selection *fieldSelection[T]) retain(choices []T) {
	if selection.choice != nil && !slices.Contains(choices, *selection.choice) {
		selection.choice = nil
	}
}

func (selection *fieldSelection[T]) cycle(choices []T, previous bool) {
	if len(choices) == 0 {
		selection.choice = nil
		return
	}
	_, index, _ := selection.current(choices)
	step := 1
	if previous {
		step = -1
		if index < 0 {
			index = 0
		}
	}
	selection.choice = &choices[(index+step+len(choices))%len(choices)]
}

func (selection *fieldSelection[T]) current(choices []T) (T, int, bool) {
	if selection.choice != nil {
		if index := slices.Index(choices, *selection.choice); index >= 0 {
			return choices[index], index, true
		}
	}
	var absent T
	return absent, -1, false
}

func (selection *fieldSelection[T]) preview(choices []T, display func(T) string) string {
	value, index, selected := selection.current(choices)
	if !selected {
		return "no choice selected"
	}
	return fmt.Sprintf("‹ %d/%d › %s", index+1, len(choices), display(value))
}
