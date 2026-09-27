package hostconfig

import (
	"bytes"
	"encoding/json"
	"errors"
)

type stringField struct {
	present bool
	value   string
}

type nullablePathField struct {
	present bool
	value   *string
}

func (field *nullablePathField) UnmarshalJSON(encoded []byte) error {
	field.present = true
	if bytes.Equal(encoded, []byte("null")) {
		return nil
	}
	var path string
	if err := json.Unmarshal(encoded, &path); err != nil {
		return err
	}
	field.value = &path
	return nil
}

func (field *stringField) UnmarshalJSON(encoded []byte) error {
	field.present = true
	if bytes.Equal(encoded, []byte("null")) {
		return errors.New("null is not a string")
	}
	return json.Unmarshal(encoded, &field.value)
}
