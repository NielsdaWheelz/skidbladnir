// Package strictjson decodes bounded protocol documents without accepting
// ambiguous object members, invalid UTF-8, unknown fields, or trailing values.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const maximumNestingDepth = 64

// Decode rejects JSON representations that encoding/json would otherwise
// normalize silently before decoding the single document into target.
func Decode(encoded []byte, target any) error {
	if !utf8.Valid(encoded) {
		return errors.New("JSON is not UTF-8")
	}
	if err := validateDocument(encoded); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("JSON does not match its schema")
	}
	if err := requireEnd(decoder); err != nil {
		return err
	}
	return nil
}

func validateDocument(encoded []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	if err := scanValue(decoder, 0); err != nil {
		return errors.New("JSON is not an unambiguous document")
	}
	return requireEnd(decoder)
}

func scanValue(decoder *json.Decoder, depth int) error {
	if depth > maximumNestingDepth {
		return errors.New("JSON nesting is too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		members := make(map[string]struct{})
		for decoder.More() {
			memberToken, err := decoder.Token()
			if err != nil {
				return err
			}
			member, ok := memberToken.(string)
			if !ok {
				return errors.New("JSON object member is not a string")
			}
			if _, duplicate := members[member]; duplicate {
				return errors.New("JSON object member is duplicated")
			}
			members[member] = struct{}{}
			if err := scanValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	want := json.Delim('}')
	if delimiter == '[' {
		want = ']'
	}
	if closing != want {
		return errors.New("invalid JSON closing delimiter")
	}
	return nil
}

func requireEnd(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("JSON has trailing data")
	}
	return nil
}
