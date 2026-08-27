package hostconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type stringField struct {
	present bool
	value   string
}

func (field *stringField) UnmarshalJSON(encoded []byte) error {
	field.present = true
	if bytes.Equal(encoded, []byte("null")) {
		return errors.New("null is not a string")
	}
	return json.Unmarshal(encoded, &field.value)
}

func decodeConfig(encoded []byte) (*configDTO, error) {
	if err := rejectDuplicateObjectMembers(encoded); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var wire *configDTO
	if err := decoder.Decode(&wire); err != nil || wire == nil {
		return nil, errors.New("host config is not canonical JSON")
	}
	if err := requireJSONEnd(decoder); err != nil {
		return nil, err
	}
	return wire, nil
}

func rejectDuplicateObjectMembers(encoded []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	if err := scanJSONValue(decoder); err != nil {
		return errors.New("host config is not canonical JSON")
	}
	return requireJSONEnd(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
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
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
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

func requireJSONEnd(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("host config has trailing JSON")
	}
	return nil
}
