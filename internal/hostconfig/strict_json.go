package hostconfig

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
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
	var wire *configDTO
	if err := strictjson.Decode(encoded, &wire); err != nil || wire == nil {
		return nil, errors.New("host config is not canonical JSON")
	}
	return wire, nil
}
