// Package strictjson decodes bounded protocol documents without accepting
// ambiguous object members, invalid UTF-8, unknown fields, or trailing values.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

const maximumNestingDepth = 64

// Decode rejects JSON representations that encoding/json would otherwise
// normalize silently before decoding the single document into target.
func Decode(encoded []byte, target any) error {
	if !utf8.Valid(encoded) {
		return errors.New("JSON is not UTF-8")
	}
	if !validUnicodeScalarEscapes(encoded) {
		return errors.New("JSON contains an invalid Unicode escape")
	}
	if err := validateDocument(encoded); err != nil {
		return err
	}
	if target == nil {
		return errors.New("JSON target is nil")
	}
	if err := validateExactSchema(encoded, reflect.TypeOf(target)); err != nil {
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

var jsonUnmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()

// validateExactSchema closes encoding/json's case-insensitive struct-field
// matching before decoding. Custom JSON unmarshallers remain the exact owner of
// their own value shape; every ordinary struct, slice, and map is traversed
// against its declared schema.
func validateExactSchema(encoded []byte, targetType reflect.Type) error {
	var document json.RawMessage = encoded
	if err := validateExactValue(document, targetType); err != nil {
		return errors.New("JSON object members do not match their exact schema")
	}
	return nil
}

func validateExactValue(encoded json.RawMessage, targetType reflect.Type) error {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType.Implements(jsonUnmarshalerType) ||
		(reflect.PointerTo(targetType).Implements(jsonUnmarshalerType)) {
		return nil
	}
	switch targetType.Kind() {
	case reflect.Struct:
		if bytes.Equal(encoded, []byte("null")) {
			return nil
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
			return errors.New("JSON value is not an object")
		}
		fields, err := exactStructFields(targetType)
		if err != nil {
			return err
		}
		for name, value := range object {
			fieldType, present := fields[name]
			if !present {
				return errors.New("JSON object member is unknown or has the wrong case")
			}
			if err := validateExactValue(value, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if bytes.Equal(encoded, []byte("null")) && targetType.Kind() == reflect.Slice {
			return nil
		}
		var values []json.RawMessage
		if err := json.Unmarshal(encoded, &values); err != nil {
			return errors.New("JSON value is not an array")
		}
		for _, value := range values {
			if err := validateExactValue(value, targetType.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		if targetType.Key().Kind() != reflect.String || bytes.Equal(encoded, []byte("null")) {
			return nil
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &values); err != nil {
			return errors.New("JSON value is not an object")
		}
		for _, value := range values {
			if err := validateExactValue(value, targetType.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func exactStructFields(targetType reflect.Type) (map[string]reflect.Type, error) {
	fields := make(map[string]reflect.Type)
	for index := 0; index < targetType.NumField(); index++ {
		field := targetType.Field(index)
		if !field.IsExported() {
			continue
		}
		tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tag == "-" {
			continue
		}
		if field.Anonymous && tag == "" {
			return nil, errors.New("embedded JSON schema fields are unsupported")
		}
		if tag == "" {
			tag = field.Name
		}
		if _, duplicate := fields[tag]; duplicate {
			return nil, errors.New("JSON schema member is duplicated")
		}
		fields[tag] = field.Type
	}
	return fields, nil
}

func validUnicodeScalarEscapes(encoded []byte) bool {
	for index := 0; index < len(encoded); {
		if encoded[index] != '"' {
			index++
			continue
		}
		index++
		closed := false
		for index < len(encoded) {
			switch encoded[index] {
			case '"':
				index++
				closed = true
			case '\\':
				if index+1 >= len(encoded) {
					return false
				}
				if encoded[index+1] != 'u' {
					index += 2
					continue
				}
				first, ok := decodeHex16(encoded, index+2)
				if !ok {
					return false
				}
				index += 6
				switch {
				case first >= 0xd800 && first <= 0xdbff:
					if index+6 > len(encoded) || encoded[index] != '\\' || encoded[index+1] != 'u' {
						return false
					}
					second, secondOK := decodeHex16(encoded, index+2)
					if !secondOK || second < 0xdc00 || second > 0xdfff {
						return false
					}
					index += 6
				case first >= 0xdc00 && first <= 0xdfff:
					return false
				}
			default:
				index++
			}
			if closed {
				break
			}
		}
		if !closed {
			return false
		}
	}
	return true
}

func decodeHex16(encoded []byte, start int) (uint16, bool) {
	if start+4 > len(encoded) {
		return 0, false
	}
	var value uint16
	for _, character := range encoded[start : start+4] {
		value <<= 4
		switch {
		case character >= '0' && character <= '9':
			value += uint16(character - '0')
		case character >= 'a' && character <= 'f':
			value += uint16(character-'a') + 10
		case character >= 'A' && character <= 'F':
			value += uint16(character-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
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
