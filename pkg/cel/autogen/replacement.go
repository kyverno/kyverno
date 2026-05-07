package autogen

import (
	"bytes"
)

var protectedMetadataFields = [][2][]byte{
	{[]byte("object.metadata.namespace"), []byte("__KYVERNO_PROTECTED_OBJECT_METADATA_NAMESPACE__")},
	{[]byte("oldObject.metadata.namespace"), []byte("__KYVERNO_PROTECTED_OLD_OBJECT_METADATA_NAMESPACE__")},
}

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
	for _, replacement := range protectedMetadataFields {
		data = bytes.ReplaceAll(data, replacement[0], replacement[1])
	}
	for _, replacement := range replacements {
		data = replacement.Apply(data)
	}
	for _, replacement := range protectedMetadataFields {
		data = bytes.ReplaceAll(data, replacement[1], replacement[0])
	}
	return data
}
