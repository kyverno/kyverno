package anchor

import (
	"testing"
)

func FuzzAnchorParseTest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		_ = Parse(data)
	})
}
