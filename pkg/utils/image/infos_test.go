package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetImageInfo(t *testing.T) {
	validateImageInfo(t,
		"nginx",
		"nginx",
		"nginx",
		"docker.io",
		"latest",
		"",
		"docker.io/nginx:latest")

	validateImageInfo(t,
		"nginx:v10.3",
		"nginx",
		"nginx",
		"docker.io",
		"v10.3",
		"",
		"docker.io/nginx:v10.3")

	validateImageInfo(t,
		"docker.io/test/nginx:v10.3",
		"nginx",
		"test/nginx",
		"docker.io",
		"v10.3",
		"",
		"docker.io/test/nginx:v10.3")

	validateImageInfo(t,
		"test/nginx",
		"nginx",
		"test/nginx",
		"docker.io",
		"latest",
		"",
		"docker.io/test/nginx:latest")

	validateImageInfo(t,
		"localhost:4443/test/nginx",
		"nginx",
		"test/nginx",
		"localhost:4443",
		"latest",
		"",
		"localhost:4443/test/nginx:latest")
	validateImageInfo(t,
		"docker.io/test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		"centos",
		"test/centos",
		"docker.io",
		"",
		"sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		"docker.io/test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f")
}

func Test_ReferenceWithTag(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{{
		input:    "nginx",
		expected: "docker.io/nginx:latest",
	}, {
		input:    "nginx:v10.3",
		expected: "docker.io/nginx:v10.3",
	}, {
		input:    "docker.io/test/nginx:v10.3",
		expected: "docker.io/test/nginx:v10.3",
	}, {
		input:    "test/nginx",
		expected: "docker.io/test/nginx:latest",
	}, {
		input:    "localhost:4443/test/nginx",
		expected: "localhost:4443/test/nginx:latest",
	}, {
		input:    "docker.io/test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		expected: "docker.io/test/centos:",
	}}
	for _, test := range testCases {
		imageInfo, err := GetImageInfo(test.input)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, imageInfo.ReferenceWithTag())
	}
}

func Test_ParseError(t *testing.T) {
	testCases := []string{
		"++",
	}
	for _, test := range testCases {
		imageInfo, err := GetImageInfo(test)
		assert.Error(t, err)
		assert.Nil(t, imageInfo)
	}
}

func validateImageInfo(t *testing.T, raw, name, path, registry, tag, digest, str string) {
	i1, err := GetImageInfo(raw)
	assert.NoError(t, err)
	assert.Equal(t, name, i1.Name)
	assert.Equal(t, path, i1.Path)
	assert.Equal(t, registry, i1.Registry)
	assert.Equal(t, tag, i1.Tag)
	assert.Equal(t, digest, i1.Digest)
	assert.Equal(t, str, i1.String())
}
