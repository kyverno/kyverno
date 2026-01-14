package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_checkResourceDescription_Subresources(t *testing.T) {
	tests := []struct {
		name          string
		kinds         []string
		gvk           schema.GroupVersionKind
		subresource   string
		expectedMatch bool
		description   string
	}{
		{
			name:          "Exception with Pod matches Pod without subresource",
			kinds:         []string{"Pod"},
			gvk:           schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			subresource:   "",
			expectedMatch: true,
			description:   "Pod should match Pod",
		},
		{
			name:          "Exception with Pod matches Pod/exec subresource",
			kinds:         []string{"Pod"},
			gvk:           schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			subresource:   "exec",
			expectedMatch: true,
			description:   "Pod should match Pod/exec when no explicit subresource in exception",
		},
		{
			name:          "Exception with Pod/exec matches Pod/exec subresource",
			kinds:         []string{"Pod/exec"},
			gvk:           schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			subresource:   "exec",
			expectedMatch: true,
			description:   "Pod/exec should match Pod/exec",
		},
		{
			name:          "Exception with Pod/exec does not match Pod without subresource",
			kinds:         []string{"Pod/exec"},
			gvk:           schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			subresource:   "",
			expectedMatch: false,
			description:   "Pod/exec should not match Pod",
		},
		{
			name:          "Exception with Pod/exec does not match Pod/log",
			kinds:         []string{"Pod/exec"},
			gvk:           schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			subresource:   "log",
			expectedMatch: false,
			description:   "Pod/exec should not match Pod/log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditionBlock := kyvernov1.ResourceDescription{
				Kinds: tt.kinds,
			}
			resource := unstructured.Unstructured{}
			resource.SetKind("Pod")
			resource.SetName("test-pod")

			result := checkResourceDescription(conditionBlock, resource, nil, tt.gvk, tt.subresource)
			assert.Equal(t, tt.expectedMatch, result, tt.description)
		})
	}
}
