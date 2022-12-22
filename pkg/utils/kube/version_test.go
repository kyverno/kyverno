package kube

import (
	"testing"

	"gotest.tools/assert"
)

func Test_higherVersion(t *testing.T) {
	v, err := isVersionHigher("invalid.version", 1, 1, 1)
	assert.Assert(t, v == false && err != nil)

	v, err = isVersionHigher("invalid-version", 0, 0, 0)
	assert.Assert(t, v == false && err != nil)

	v, err = isVersionHigher("v1.1.1", 1, 1, 1)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v1.0.0", 1, 1, 1)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v1.5.9", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9+distro", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9+distro", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9-rc2", 1, 5, 9)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v1.5.9", 2, 1, 0)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v2.1.0", 1, 5, 9)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9-x-v1.5.9.x", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)
}
