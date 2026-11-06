package compiler

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Match(t *testing.T) {
	tests := []struct {
		name           string
		imageExtractor []v1beta1.ImageExtractor
		request        map[string]any
		gvr            *metav1.GroupVersionResource
		wantResult     map[string][]string
		wantErr        bool
	}{{
		name: "standard",
		imageExtractor: []v1beta1.ImageExtractor{{
			Name:       "one",
			Expression: "request.images",
		}},
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
	}, {
		name: "pod image extraction",
		imageExtractor: []v1beta1.ImageExtractor{{
			Name:       "one",
			Expression: "request.images",
		}},
		request: map[string]any{
			"request": map[string]any{
				"images": []string{
					"nginx:latest",
					"alpine:latest",
				},
			},
			"object": map[string]any{
				"spec": map[string]any{
					"containers": []map[string]string{{
						"image": "kyverno/image-one",
					}, {
						"image": "kyverno/image-two",
					}},
					"initContainers": []map[string]string{{
						"image": "kyverno/init-image-one",
					}, {
						"image": "kyverno/init-image-two",
					}},
					"ephemeralContainers": []map[string]string{{
						"image": "kyverno/ephr-image-one",
					}, {
						"image": "kyverno/ephr-image-two",
					}},
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
			"imageVolumes": {},
		},
		gvr:     &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		wantErr: false,
	}, {
		name: "pod image extraction (old object)",
		imageExtractor: []v1beta1.ImageExtractor{{
			Name:       "one",
			Expression: "request.images",
		}},
		request: map[string]any{
			"request": map[string]any{
				"images": []string{
					"nginx:latest",
					"alpine:latest",
				},
			},
			"object": nil,
			"oldObject": map[string]any{
				"spec": map[string]any{
					"containers": []map[string]string{{
						"image": "kyverno/image-one",
					}, {
						"image": "kyverno/image-two",
					}},
					"initContainers": []map[string]string{{
						"image": "kyverno/init-image-one",
					}, {
						"image": "kyverno/init-image-two",
					}},
					"ephemeralContainers": []map[string]string{{
						"image": "kyverno/ephr-image-one",
					}, {
						"image": "kyverno/ephr-image-two",
					}},
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
			"imageVolumes": {},
		},
		gvr:     &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		wantErr: false,
	}, {
		name: "standard fail",
		imageExtractor: []v1beta1.ImageExtractor{{
			Name:       "one",
			Expression: "request.images",
		}},
		request: map[string]any{
			"request": map[string][]int{
				"images": {0, 1},
			},
		},
		gvr:     nil,
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, err := NewBaseEnv()
			assert.NoError(t, err)
			env, err := base.Extend(
				cel.Variable(RequestKey, types.DynType),
				cel.Variable(ObjectKey, types.DynType),
				cel.Variable(OldObjectKey, types.DynType),
			)
			c, errs := CompileImageExtractors(
				field.NewPath("spec", "images"),
				env,
				tt.gvr,
				tt.imageExtractor...,
			)
			assert.Nil(t, errs)
			images, err := ExtractImages(context.Background(), tt.request, c)
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

// Image volumes are extracted for every resource the built-in extractors cover, at
// each resource's own pod template path. Volumes without an image are skipped.
func Test_ImageVolumes(t *testing.T) {
	podSpec := func() map[string]any {
		return map[string]any{
			"containers": []any{map[string]any{"image": "kyverno/app"}},
			"volumes": []any{
				map[string]any{"name": "model", "image": map[string]any{"reference": "kyverno/model:v1"}},
				map[string]any{"name": "config", "configMap": map[string]any{"name": "cm"}},
				map[string]any{"name": "scratch", "emptyDir": map[string]any{}},
				map[string]any{"name": "extra-model", "image": map[string]any{"reference": "kyverno/model:v2", "pullPolicy": "IfNotPresent"}},
				// Workload templates may leave the reference to config management, and CLI input
				// can carry nulls: none of these may stop the valid references from being read.
				map[string]any{"name": "null-image", "image": nil},
				map[string]any{"name": "no-reference", "image": map[string]any{"pullPolicy": "Always"}},
				map[string]any{"name": "empty-image", "image": map[string]any{}},
				map[string]any{"name": "empty-reference", "image": map[string]any{"reference": ""}},
				map[string]any{"name": "null-reference", "image": map[string]any{"reference": nil}},
				map[string]any{"name": "blank-reference", "image": map[string]any{"reference": "  "}},
			},
		}
	}
	template := func(spec map[string]any) map[string]any {
		return map[string]any{"spec": map[string]any{"template": map[string]any{"spec": spec}}}
	}
	tests := []struct {
		name   string
		gvr    metav1.GroupVersionResource
		object map[string]any
	}{
		{"pod", pods, map[string]any{"spec": podSpec()}},
		{"deployment", deployments, template(podSpec())},
		{"statefulset", statefulsets, template(podSpec())},
		{"daemonset", daemonsets, template(podSpec())},
		{"replicaset", replicasets, template(podSpec())},
		{"replicationcontroller", replicationcontrollers, template(podSpec())},
		{"job", jobs, template(podSpec())},
		{"cronjob", cronjobs, map[string]any{"spec": map[string]any{"jobTemplate": template(podSpec())}}},
	}
	base, err := NewBaseEnv()
	assert.NoError(t, err)
	env, err := base.Extend(
		cel.Variable(ObjectKey, types.DynType),
		cel.Variable(OldObjectKey, types.DynType),
	)
	assert.NoError(t, err)
	for _, tt := range tests {
		for _, deleting := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s deleting=%v", tt.name, deleting), func(t *testing.T) {
				gvr := tt.gvr
				extractors, errs := CompileImageExtractors(field.NewPath("spec", "images"), env, &gvr)
				assert.Nil(t, errs)
				data := map[string]any{ObjectKey: tt.object, OldObjectKey: nil}
				if deleting {
					data[ObjectKey], data[OldObjectKey] = nil, tt.object
				}
				images, err := ExtractImages(context.Background(), data, extractors)
				assert.NoError(t, err)
				assert.Equal(t, []string{"kyverno/model:v1", "kyverno/model:v2"}, images["imageVolumes"])
				assert.Equal(t, []string{"kyverno/app"}, images["containers"])
			})
		}
	}
	t.Run("null volumes", func(t *testing.T) {
		gvr := pods
		extractors, errs := CompileImageExtractors(field.NewPath("spec", "images"), env, &gvr)
		assert.Nil(t, errs)
		images, err := ExtractImages(context.Background(), map[string]any{
			ObjectKey: map[string]any{"spec": map[string]any{
				"containers": []any{map[string]any{"image": "kyverno/app"}},
				"volumes":    nil,
			}},
			OldObjectKey: nil,
		}, extractors)
		assert.NoError(t, err)
		assert.Empty(t, images["imageVolumes"])
		assert.Equal(t, []string{"kyverno/app"}, images["containers"])
	})
	t.Run("no volumes", func(t *testing.T) {
		gvr := pods
		extractors, errs := CompileImageExtractors(field.NewPath("spec", "images"), env, &gvr)
		assert.Nil(t, errs)
		images, err := ExtractImages(context.Background(), map[string]any{
			ObjectKey:    map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"image": "kyverno/app"}}}},
			OldObjectKey: nil,
		}, extractors)
		assert.NoError(t, err)
		assert.Empty(t, images["imageVolumes"])
	})
}
