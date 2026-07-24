package json

import (
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"gotest.tools/assert"
)

func Test_JoinPatches(t *testing.T) {
	patches := JoinPatches()
	assert.Assert(t, patches == nil, "invalid patch %#v", string(patches))

	patches = JoinPatches([]byte(""))
	assert.Assert(t, patches == nil, "invalid patch %#v", string(patches))

	patches = JoinPatches([]byte(""), []byte(""), []byte(""), []byte(""))
	assert.Assert(t, patches == nil, "invalid patch %#v", string(patches))

	p1 := `{ "op": "replace", "path": "/baz", "value": "boo" }`
	p2 := `{ "op": "add", "path": "/hello", "value": ["world"] }`
	p1p2 := `[
		{ "op": "replace", "path": "/baz", "value": "boo" },
		{ "op": "add", "path": "/hello", "value": ["world"] }
	]`

	patches = JoinPatches([]byte(p1), []byte(p2))
	_, err := jsonpatch.DecodePatch(patches)
	assert.NilError(t, err, "failed to decode patch %s", string(patches))
	if !jsonpatch.Equal([]byte(p1p2), patches) {
		assert.Assert(t, false, "patches are not equal")
	}

	p3 := `{ "op": "remove", "path": "/foo" }`
	p1p2p3 := `[
		{ "op": "replace", "path": "/baz", "value": "boo" },
		{ "op": "add", "path": "/hello", "value": ["world"] },
		{ "op": "remove", "path": "/foo" }
	]`

	patches = JoinPatches([]byte(p1p2), []byte(p3))
	assert.NilError(t, err, "failed to join patches %s", string(patches))

	_, err = jsonpatch.DecodePatch(patches)
	assert.NilError(t, err, "failed to decode patch %s", string(patches))
	if !jsonpatch.Equal([]byte(p1p2p3), patches) {
		assert.Assert(t, false, "patches are not equal %+v %+v", p1p2p3, string(patches))
	}
}

func Test_JoinPatches_EmptyArray(t *testing.T) {
	p := `{ "op": "add", "path": "/hello", "value": ["world"] }`
	expected := `[{ "op": "add", "path": "/hello", "value": ["world"] }]`

	// An empty patch array must be dropped, not turned into an empty operation
	// that leaves a stray comma and malformed JSON (#16710).
	patches := JoinPatches([]byte(`[]`), []byte(p))
	_, err := jsonpatch.DecodePatch(patches)
	assert.NilError(t, err, "failed to decode patch %s", string(patches))
	if !jsonpatch.Equal([]byte(expected), patches) {
		assert.Assert(t, false, "patches are not equal, got %s", string(patches))
	}

	// The empty array in the trailing position must not leave a trailing comma either.
	patches = JoinPatches([]byte(p), []byte(`[]`))
	_, err = jsonpatch.DecodePatch(patches)
	assert.NilError(t, err, "failed to decode patch %s", string(patches))
	if !jsonpatch.Equal([]byte(expected), patches) {
		assert.Assert(t, false, "patches are not equal, got %s", string(patches))
	}

	// Only empty arrays collapse to nil, consistent with empty strings.
	patches = JoinPatches([]byte(`[]`), []byte(`[]`))
	assert.Assert(t, patches == nil, "expected nil, got %#v", string(patches))
}
