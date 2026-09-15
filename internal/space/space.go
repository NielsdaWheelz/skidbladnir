// Package space owns session membership and collection selection.
package space

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var ErrInvalid = errors.New("use 1–64 nfc characters; only interior ordinary spaces, without display controls.")

// Label is canonical text; its zero value is unassigned.
type Label struct{ text string }

func Parse(text string) (Label, error) {
	if text == "" {
		return Label{}, nil
	}
	if !utf8.ValidString(text) || len(text) > 256 || utf8.RuneCountInString(text) > 64 ||
		normalizeNFC(text) != text || text[0] == ' ' || text[len(text)-1] == ' ' {
		return Label{}, ErrInvalid
	}
	for _, value := range text {
		if value <= 0x1f || value >= 0x7f && value <= 0x9f || value == 0x061c ||
			value >= 0x200e && value <= 0x200f || value >= 0x2028 && value <= 0x202e ||
			value >= 0x2066 && value <= 0x2069 || value == 0xa0 || value == 0x1680 ||
			value >= 0x2000 && value <= 0x200a || value == 0x202f || value == 0x205f || value == 0x3000 {
			return Label{}, ErrInvalid
		}
	}
	return Label{text: text}, nil
}

func ParseDraft(text string) (Label, error) {
	if !utf8.ValidString(text) {
		return Label{}, ErrInvalid
	}
	return Parse(normalizeNFC(text))
}

// normalizeNFC implements plain NFC. Whole-string x/text normalization inserts
// CGJ after 30 nonstarters for stream safety, changing otherwise valid labels.
// Scalar decomposition and pair composition reuse its Unicode tables without
// that insertion; ordering and blocking follow Unicode UAX #15.
func normalizeNFC(text string) string {
	type scalar struct {
		value rune
		class uint8
	}
	decomposed := make([]scalar, 0, len(text))
	for _, value := range text {
		for _, value := range norm.NFD.String(string(value)) {
			decomposed = append(decomposed, scalar{value, norm.NFD.PropertiesString(string(value)).CCC()})
		}
	}
	for start := 0; start < len(decomposed); {
		if decomposed[start].class == 0 {
			start++
			continue
		}
		end := start + 1
		for end < len(decomposed) && decomposed[end].class != 0 {
			end++
		}
		slices.SortStableFunc(decomposed[start:end], func(left, right scalar) int {
			return int(left.class) - int(right.class)
		})
		start = end
	}
	composed := make([]rune, 0, len(decomposed))
	starter := -1
	var blockingClass uint8
	for _, value := range decomposed {
		if starter >= 0 && (blockingClass == 0 || blockingClass < value.class) {
			pair := norm.NFC.String(string(composed[starter]) + string(value.value))
			combined, size := utf8.DecodeRuneInString(pair)
			if size == len(pair) {
				composed[starter] = combined
				continue
			}
		}
		if value.class == 0 {
			starter = len(composed)
		}
		composed = append(composed, value.value)
		blockingClass = value.class
	}
	return string(composed)
}

func (label Label) String() string     { return label.text }
func (label Label) IsUnassigned() bool { return label.text == "" }

func Compare(left, right Label) int {
	if left.IsUnassigned() && !right.IsUnassigned() {
		return 1
	}
	if !left.IsUnassigned() && right.IsUnassigned() {
		return -1
	}
	for index := 0; index < min(len(left.text), len(right.text)); index++ {
		a, b := left.text[index], right.text[index]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	}
	if len(left.text) < len(right.text) {
		return -1
	}
	if len(left.text) > len(right.text) {
		return 1
	}
	return strings.Compare(left.text, right.text)
}

type FilterKind uint8

const (
	FilterAll FilterKind = iota
	FilterUnassigned
	FilterNamed
)

// Filter's zero value selects all memberships.
type Filter struct {
	kind  FilterKind
	label Label
}

func UnassignedFilter() Filter { return Filter{kind: FilterUnassigned} }
func NamedFilter(label Label) (Filter, error) {
	if label.IsUnassigned() {
		return Filter{}, ErrInvalid
	}
	return Filter{kind: FilterNamed, label: label}, nil
}
func (filter Filter) Kind() FilterKind { return filter.kind }
func (filter Filter) Label() Label     { return filter.label }
func (filter Filter) Matches(label Label) bool {
	switch filter.kind {
	case FilterAll:
		return true
	case FilterUnassigned:
		return label.IsUnassigned()
	case FilterNamed:
		return label == filter.label
	default:
		panic("invalid space filter") // justify-defect: only the closed constructors populate filter kinds.
	}
}
