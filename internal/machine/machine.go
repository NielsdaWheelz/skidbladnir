package machine

import "errors"

const handlePrefix = "mh-"

type Handle struct {
	value string
}

func Parse(value string) (Handle, error) {
	if len(value) != len(handlePrefix)+32 || value[:len(handlePrefix)] != handlePrefix {
		return Handle{}, errors.New("machine handle is not canonical")
	}
	for _, character := range value[len(handlePrefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return Handle{}, errors.New("machine handle is not canonical")
		}
	}
	return Handle{value: value}, nil
}

func (handle Handle) String() string {
	return handle.value
}
