package variables

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Match(t *testing.T) {
	tests := []struct {
		name           string
		imageExtractor []v1alpha1.Image
		request        map[string]any
		gvr            *metav1.GroupVersionResource
		wantResult     map[string][]string
		wantErr        bool
	}{
		{
			name: "standard",
			imageExtractor: []v1alpha1.Image{
				{
					Name:       "one",
					Expression: "request.images",
				},
			},
			request: map[string]any{
				"request": map[string][]string{
					"images": {
						"nginx:latest",
						"alpine:latest",
					},
				},
			},
			wantResult: map[string][]string{
				"one": {
					"nginx:latest",
					"alpine:latest",
				},
			},
			gvr:     nil,
			wantErr: false,
		},
		{
			name: "pod image extraction",
			imageExtractor: []v1alpha1.Image{
				{
					Name:       "one",
					Expression: "request.images",
				},
			},
			request: map[string]any{
				"request": map[string]any{
					"images": []string{
						"nginx:latest",
						"alpine:latest",
					},
					"object": map[string]any{
						"spec": map[string]any{
							"containers": []map[string]string{
								{
									"image": "kyverno/image-one",
								},
								{
									"image": "kyverno/image-two",
								},
							},
							"initContainers": []map[string]string{
								{
									"image": "kyverno/init-image-one",
								},
								{
									"image": "kyverno/init-image-two",
								},
							},
							"ephemeralContainers": []map[string]string{
								{
									"image": "kyverno/ephr-image-one",
								},
								{
									"image": "kyverno/ephr-image-two",
								},
							},
						},
					},
				},
			},
			wantResult: map[string][]string{
				"one": {
					"nginx:latest",
					"alpine:latest",
				},
				"containers": {
					"kyverno/image-one",
					"kyverno/image-two",
				},
				"initContainers": {
					"kyverno/init-image-one",
					"kyverno/init-image-two",
				},
				"ephemeralContainers": {
					"kyverno/ephr-image-one",
					"kyverno/ephr-image-two",
				},
			},
			gvr:     &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			wantErr: false,
		},
		{
			name: "standard fail",
			imageExtractor: []v1alpha1.Image{
				{
					Name:       "one",
					Expression: "request.images",
				},
			},
			request: map[string]any{
				"request": map[string][]int{
					"images": {0, 1},
				},
			},
			gvr:     nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, errList := CompileImageExtractors(field.NewPath("spec", "images"), tt.imageExtractor, tt.gvr, []cel.EnvOption{cel.Variable(policy.RequestKey, types.DynType)})
			assert.Nil(t, errList)
			images, err := ExtractImages(c, tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				cmp := func(a, b map[string][]string) bool {
					if len(a) != len(b) {
						return false
					}
					for k, v := range a {
						if w, ok := b[k]; !ok || !slices.Equal(v, w) {
							return false
						}
					}
					return true
				}

				assert.NoError(t, err)
				assert.True(t, cmp(tt.wantResult, images), fmt.Sprintf("want=%v, got=%v", tt.wantResult, images))
			}
		})
	}
}
