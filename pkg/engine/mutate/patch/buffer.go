package patch

import (
	"bytes"
)

// buffer is a wrapper around a slice of bytes used for JSON
// marshal and unmarshal operations for a strategic merge patch
type buffer struct {
	*bytes.Buffer
}

// UnmarshalJSON writes the slice of bytes to an internal buffer
func (buff buffer) UnmarshalJSON(b []byte) error {
	buff.Reset()
	if _, err := buff.Write(b); err != nil {
		return err
	}
	return nil
}

// MarshalJSON returns the buffered slice of bytes. The returned slice
// is valid for use only until the next buffer modification.
func (buff buffer) MarshalJSON() ([]byte, error) {
	return buff.Bytes(), nil
}
