package image

import (
	"strconv"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// initializeMockConfig initializes a basic configuration with a fake dynamic client
func initializeMockConfig(defaultRegistry string, enableDefaultRegistryMutation bool) (config.Configuration, error) {
	configMapData := make(map[string]string, 0)
	configMapData["defaultRegistry"] = defaultRegistry
	configMapData["enableDefaultRegistryMutation"] = strconv.FormatBool(enableDefaultRegistryMutation)
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "kyverno"},
		Data:       configMapData,
	}
	dynamicConfig := config.NewDefaultConfiguration(false)
	dynamicConfig.Load(&cm)
	return dynamicConfig, nil
}

func Test_GetImageInfo(t *testing.T) {
	validateImageInfo(t,
		"nginx",
		"nginx",
		"nginx",
		"docker.io",
		"latest",
		"",
		"docker.io/nginx:latest",
		"docker.io",
		true)

	validateImageInfo(t,
		"nginx:v10.3",
		"nginx",
		"nginx",
		"docker.io",
		"v10.3",
		"",
		"docker.io/nginx:v10.3",
		"docker.io",
		true)

	validateImageInfo(t,
		"docker.io/test/nginx:v10.3",
		"nginx",
		"test/nginx",
		"docker.io",
		"v10.3",
		"",
		"docker.io/test/nginx:v10.3",
		"docker.io",
		true)

	validateImageInfo(t,
		"test/nginx",
		"nginx",
		"test/nginx",
		"docker.io",
		"latest",
		"",
		"docker.io/test/nginx:latest",
		"docker.io",
		true)

	validateImageInfo(t,
		"localhost:4443/test/nginx",
		"nginx",
		"test/nginx",
		"localhost:4443",
		"latest",
		"",
		"localhost:4443/test/nginx:latest",
		"docker.io",
		true)

	validateImageInfo(t,
		"docker.io/test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		"centos",
		"test/centos",
		"docker.io",
		"",
		"sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		"docker.io/test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		"docker.io",
		true)

	validateImageInfo(t,
		"test/nginx",
		"nginx",
		"test/nginx",
		"gcr.io",
		"latest",
		"",
		"gcr.io/test/nginx:latest",
		"gcr.io",
		true)

	validateImageInfo(t,
		"test/nginx",
		"nginx",
		"test/nginx",
		"",
		"latest",
		"",
		"test/nginx:latest",
		"gcr.io",
		false)
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
	cfg, err := initializeMockConfig("docker.io", true)
	assert.NoError(t, err)
	for _, test := range testCases {
		imageInfo, err := GetImageInfo(test.input, cfg)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, imageInfo.ReferenceWithTag())
	}
}

func Test_ParseError(t *testing.T) {
	testCases := []string{
		"++",
	}
	cfg, err := initializeMockConfig("docker.io", true)
	assert.NoError(t, err)
	for _, test := range testCases {

		imageInfo, err := GetImageInfo(test, cfg)
		assert.Error(t, err)
		assert.Nil(t, imageInfo)
	}
}

func validateImageInfo(t *testing.T, raw, name, path, registry, tag, digest, str string, defautRegistry string, enableDefaultRegistryMutation bool) {
	cfg, err := initializeMockConfig(defautRegistry, enableDefaultRegistryMutation)
	assert.NoError(t, err)
	i1, err := GetImageInfo(raw, cfg)
	assert.NoError(t, err)
	assert.Equal(t, name, i1.Name)
	assert.Equal(t, path, i1.Path)
	assert.Equal(t, registry, i1.Registry)
	assert.Equal(t, tag, i1.Tag)
	assert.Equal(t, digest, i1.Digest)
	assert.Equal(t, str, i1.String())
}

func Test_addDefaultRegistry(t *testing.T) {
	tests := []struct {
		input                         string
		defaultRegistry               string
		enableDefaultRegistryMutation bool
		want                          string
	}{
		{
			defaultRegistry:               "test.io",
			enableDefaultRegistryMutation: true,
			input:                         "docker.io/test/nginx:v10.4",
			want:                          "docker.io/test/nginx:v10.4",
		},
		{
			defaultRegistry:               "docker.io",
			enableDefaultRegistryMutation: true,
			input:                         "test/nginx:v10.3",
			want:                          "docker.io/test/nginx:v10.3",
		},
		{
			defaultRegistry:               "myregistry.io",
			enableDefaultRegistryMutation: false,
			input:                         "test/nginx:v10.6",
			want:                          "myregistry.io/test/nginx:v10.6",
		},
		{
			input:                         "localhost/netd:v0.4.4-gke.0",
			defaultRegistry:               "docker.io",
			enableDefaultRegistryMutation: true,
			want:                          "localhost/netd:v0.4.4-gke.0",
		},
		{
			input:                         "myregistry.org/test/nginx:v10.3",
			defaultRegistry:               "docker.io",
			enableDefaultRegistryMutation: false,
			want:                          "myregistry.org/test/nginx:v10.3",
		},
		{
			input:                         "test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
			defaultRegistry:               "docker.io",
			enableDefaultRegistryMutation: true,
			want:                          "docker.io/test/centos@sha256:dead07b4d8ed7e29e98de0f4504d87e8880d4347859d839686a31da35a3b532f",
		},
	}

	for _, tt := range tests {
		cfg, err := initializeMockConfig(tt.defaultRegistry, true)
		assert.NoError(t, err)
		got := addDefaultRegistry(tt.input, cfg)
		assert.Equal(t, tt.want, got)
	}
}
