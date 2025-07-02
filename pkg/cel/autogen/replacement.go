package autogen

import (
	"bytes"
)

type Replacement struct {
	From string
	To   string
}

func (r *Replacement) Apply(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("object."+r.From), []byte("object."+r.To))
	data = bytes.ReplaceAll(data, []byte("oldObject."+r.From), []byte("oldObject."+r.To))
	return data
}

func Apply(data []byte, replacements ...Replacement) []byte {
	for _, replacement := range replacements {
		data = replacement.Apply(data)
	}
	return data
}
