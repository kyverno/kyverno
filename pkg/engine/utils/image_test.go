package utils

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestImageMatches(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		patterns []string
		want     bool
	}{
		{"exact match", "nginx:latest", []string{"nginx:latest"}, true},
		{"wildcard match", "nginx:latest", []string{"nginx:*"}, true},
		{"registry match", "ghcr.io/repo/img:v1", []string{"ghcr.io/*"}, true},
		{"no match", "redis:6", []string{"nginx:*"}, false},
		{"multiple patterns", "busybox:1.36", []string{"nginx:*", "busybox:*"}, true},
		{"empty patterns", "nginx:latest", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ImageMatches(tt.image, tt.patterns))
		})
	}
}

func TestIsImageVerified(t *testing.T) {
	log := logr.Discard()

	t.Run("nil resource", func(t *testing.T) {
		res := unstructured.Unstructured{Object: nil}
		_, err := IsImageVerified(res, "nginx", log)
		assert.Error(t, err)
	})

	t.Run("no annotations", func(t *testing.T) {
		res := unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
		}
		status, err := IsImageVerified(res, "nginx", log)
		assert.NoError(t, err)
		assert.Equal(t, engineapi.ImageVerificationFail, status)
	})

	t.Run("missing specific annotation", func(t *testing.T) {
		res := unstructured.Unstructured{}
		res.SetAnnotations(map[string]string{"other": "data"})
		status, err := IsImageVerified(res, "nginx", log)
		assert.Error(t, err)
		assert.Equal(t, engineapi.ImageVerificationFail, status)
	})

	t.Run("invalid metadata format", func(t *testing.T) {
		res := unstructured.Unstructured{}
		res.SetAnnotations(map[string]string{
			kyverno.AnnotationImageVerify: "invalid-json",
		})
		status, err := IsImageVerified(res, "nginx", log)
		assert.Error(t, err)
		assert.Equal(t, engineapi.ImageVerificationFail, status)
	})

	t.Run("successfully verified", func(t *testing.T) {
		res := unstructured.Unstructured{}
		res.SetAnnotations(map[string]string{
			kyverno.AnnotationImageVerify: `{"nginx": "pass"}`,
		})
		status, err := IsImageVerified(res, "nginx", log)
		assert.NoError(t, err)
		assert.Equal(t, engineapi.ImageVerificationPass, status)
	})
}
