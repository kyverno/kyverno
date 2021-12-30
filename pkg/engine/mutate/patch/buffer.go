package patch

import "bytes"

type buffer struct {
	*bytes.Buffer
}

func (buff buffer) UnmarshalJSON(b []byte) error {
	buff.Reset()
	_, err := buff.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (buff buffer) MarshalJSON() ([]byte, error) {
	return buff.Bytes(), nil
}
