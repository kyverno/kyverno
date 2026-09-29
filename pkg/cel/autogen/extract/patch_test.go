package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gomodules.xyz/jsonpatch/v2"
)

func TestJSONPointerPrefix_Escaping(t *testing.T) {
	e := Extracted{Segments: []PathSegment{
		{Key: "spec"},
		{Key: "weird/key~name"},
		{Index: 2, IsIdx: true},
	}}
	assert.Equal(t, "/spec/weird~1key~0name/2", e.JSONPointerPrefix())
}

func TestJSONPointerPrefix_Empty(t *testing.T) {
	e := Extracted{}
	assert.Equal(t, "", e.JSONPointerPrefix())
}

func TestRebasePatch(t *testing.T) {
	ops := []jsonpatch.JsonPatchOperation{
		jsonpatch.NewOperation("add", "/metadata/labels/team", "platform"),
		jsonpatch.NewOperation("replace", "/spec/containers/0/image", "bash:2.0"),
	}
	rebased := RebasePatch(ops, "/spec/replicatedJobs/0/template/spec/template")

	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template/metadata/labels/team", rebased[0].Path)
	assert.Equal(t, "add", rebased[0].Operation)
	assert.Equal(t, "platform", rebased[0].Value)

	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template/spec/containers/0/image", rebased[1].Path)

	// original ops must be untouched (no aliasing bugs)
	assert.Equal(t, "/metadata/labels/team", ops[0].Path)
}
