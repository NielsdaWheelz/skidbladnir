package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

var characterKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(?:\.[a-z0-9][a-z0-9-]*)+$`)

type Character struct {
	Key         string
	DisplayName string
	NameSuffix  string
}

type Catalogue struct {
	characters []Character
	byKey      map[string]Character
}

func Load(path string) (Catalogue, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Catalogue{}, fmt.Errorf("read character catalogue: %w", err)
	}
	return decode(contents)
}

func decode(contents []byte) (Catalogue, error) {
	if !utf8.Valid(contents) {
		return Catalogue{}, errors.New("character catalogue is not valid UTF-8")
	}
	var raw []struct {
		Key         string `json:"key"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(contents, &raw); err != nil {
		return Catalogue{}, fmt.Errorf("decode character catalogue: %w", err)
	}
	if len(raw) == 0 {
		return Catalogue{}, errors.New("character catalogue is empty")
	}

	characters := make([]Character, 0, len(raw))
	byKey := make(map[string]Character, len(raw))
	displayNames := make(map[string]struct{}, len(raw))
	suffixes := make(map[string]struct{}, len(raw))
	for index, entry := range raw {
		if !characterKeyPattern.MatchString(entry.Key) {
			return Catalogue{}, fmt.Errorf("character catalogue entry %d has an invalid key", index)
		}
		if !utf8.ValidString(entry.DisplayName) || strings.TrimSpace(entry.DisplayName) == "" {
			return Catalogue{}, fmt.Errorf("character catalogue entry %d has an invalid display name", index)
		}
		if _, found := byKey[entry.Key]; found {
			return Catalogue{}, fmt.Errorf("character catalogue entry %d duplicates a key", index)
		}
		if _, found := displayNames[entry.DisplayName]; found {
			return Catalogue{}, fmt.Errorf("character catalogue entry %d duplicates a display name", index)
		}
		suffix := entry.Key[strings.LastIndexByte(entry.Key, '.')+1:]
		if len("ga-"+suffix) > 64 {
			return Catalogue{}, fmt.Errorf("character catalogue entry %d has an oversized session suffix", index)
		}
		if _, found := suffixes[suffix]; found {
			return Catalogue{}, fmt.Errorf("character catalogue entry %d duplicates a session suffix", index)
		}

		character := Character{Key: entry.Key, DisplayName: entry.DisplayName, NameSuffix: suffix}
		characters = append(characters, character)
		byKey[character.Key] = character
		displayNames[character.DisplayName] = struct{}{}
		suffixes[character.NameSuffix] = struct{}{}
	}
	return Catalogue{characters: characters, byKey: byKey}, nil
}

func (catalogue Catalogue) Characters() []Character {
	return append([]Character(nil), catalogue.characters...)
}

func (catalogue Catalogue) Character(key string) (Character, bool) {
	character, found := catalogue.byKey[key]
	return character, found
}
