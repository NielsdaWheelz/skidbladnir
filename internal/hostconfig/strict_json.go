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

func (field *stringField) UnmarshalJSON(encoded []byte) error {
	field.present = true
	if bytes.Equal(encoded, []byte("null")) {
		return errors.New("null is not a string")
	}
	return json.Unmarshal(encoded, &field.value)
}
