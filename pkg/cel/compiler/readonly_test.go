package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindWriteCall(t *testing.T) {
	t.Parallel()
	tests := []struct {
		expression string
		want       string
	}{
		{`resource.post("v1", "configmaps", {})`, "resource.post"},
		{`resource.Post("v1", "configmaps", {})`, "resource.Post"},
		{`has(http.post("https://example.com", {}).status)`, "http.post"},
		{`http.Post("https://example.com", {})`, "http.Post"},
		{`http.Client("").Post("https://example.com", {})`, "http.Post"},
		{`resource.get("v1", "configmaps", "default", "cm")`, ""},
		{`object.metadata.name == "post"`, ""},
		{`resource.post(`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, FindWriteCall(tt.expression))
		})
	}
}
