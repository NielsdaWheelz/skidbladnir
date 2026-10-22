package catalog

import (
	_ "embed"
	"encoding/json"
	"regexp"
)

var characterKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(?:\.[a-z0-9][a-z0-9-]*)+$`)

// Entry is one Dvergatal character.
type Entry struct {
	Key         string
	DisplayName string
}

//go:embed characters.json
var encoded []byte

// characters is the append-only Dvergatal in file order; normalization and
// creation select by that order. Keys are unique dotted lowercase tokens, so
// inventory's token grammar admits them.
var characters, byKey = decode()

// decode admits the embedded file. Nothing else checks it, so a malformed file
// fails every command at start instead of reaching tmux options or checkpoints.
func decode() ([]Entry, map[string]Entry) {
	var decoded []Entry
	if err := json.Unmarshal(encoded, &decoded); err != nil || len(decoded) == 0 {
		panic("character catalogue is undecodable or empty") // justify-defect: the file is a build input.
	}
	keyed := make(map[string]Entry, len(decoded))
	for _, character := range decoded {
		if _, duplicate := keyed[character.Key]; duplicate || !characterKeyPattern.MatchString(character.Key) {
			panic("character catalogue key is invalid or duplicated") // justify-defect: the file is a build input.
		}
		keyed[character.Key] = character
	}
	return decoded, keyed
}

// Characters returns the shared catalogue in file order; callers never modify it.
func Characters() []Entry { return characters }

func Character(key string) (Entry, bool) {
	character, found := byKey[key]
	return character, found
}
