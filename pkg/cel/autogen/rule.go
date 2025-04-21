package autogen

import (
	"bytes"
)

func updateFields(data []byte, replacements ...replacement) []byte {
	for _, replacement := range replacements {
		data = bytes.ReplaceAll(data, []byte("object."+replacement.from), []byte("object."+replacement.to))
		data = bytes.ReplaceAll(data, []byte("oldObject."+replacement.from), []byte("oldObject."+replacement.to))
	}
	return data
}
