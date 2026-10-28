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
	"github.com/stretchr/testify/require"
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

func TestImageResourcePaths(test *testing.T) {
	environment, err := cel.NewEnv(
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("variables", cel.DynType),
		cel.OptionalTypes(),
	)
	require.NoError(test, err)
	for _, testCase := range []struct {
		expression string
		paths      [][]string
	}{
		{expression: `["author"]`},
		{expression: `[object.spec.artifact]`, paths: [][]string{{"object", "spec", "artifact"}}},
		{expression: `[oldObject.spec.artifacts[12]]`, paths: [][]string{{"oldObject", "spec", "artifacts", "12"}}},
		{expression: `[variables.image]`, paths: [][]string{{"variables", "image"}}},
		{expression: `[object["spec"]["artifact"]]`, paths: [][]string{{"object", "spec", "artifact"}}},
		{expression: `object.metadata.name == "x" ? ["author"] : [object.spec.artifact]`, paths: [][]string{{"object", "spec", "artifact"}}},
		{expression: `object.spec.artifact == "x" ? ["author"] : ["other"]`},
		{expression: `[object.spec.first] + [oldObject.spec.second]`, paths: [][]string{{"object", "spec", "first"}, {"oldObject", "spec", "second"}}},
		{expression: `[object.spec.artifact + ":tag"]`},
		{expression: `[string(object.spec.artifact)]`},
		{expression: `[object.spec.?artifact.orValue("author")]`, paths: [][]string{{"object", "spec", "artifact"}}},
		{expression: `[object.spec.artifacts[?0].orValue("author")]`, paths: [][]string{{"object", "spec", "artifacts", "0"}}},
		{expression: `object.spec.artifacts.map(entry, entry.image)`, paths: [][]string{{"object", "spec", "artifacts", "*", "image"}}},
		{expression: `object.spec.artifacts.map(entry, "author")`},
		{expression: `object.spec.artifacts.map(entry, entry.enabled, "author")`},
		{expression: `object.spec.artifacts.map(entry, entry.enabled, entry.image)`, paths: [][]string{{"object", "spec", "artifacts", "*", "image"}}},
		{expression: `object.spec.artifacts.filter(entry, entry.enabled).map(entry, entry.image)`, paths: [][]string{{"object", "spec", "artifacts", "*", "image"}}},
		{expression: `[object.spec.artifact].map(entry, entry)`, paths: [][]string{{"object", "spec", "artifact"}}},
		{expression: `(object != null ? object : oldObject).spec.?containers.orValue([]).map(entry, entry.image)`, paths: [][]string{{"object", "spec", "containers", "*", "image"}, {"oldObject", "spec", "containers", "*", "image"}}},
		{expression: `[("author" in object.spec.artifacts)]`},
		{expression: `[has(object.spec.artifact)]`},
		{expression: `[["author", object.spec.artifact][0]]`},
		{expression: `[(object != null ? ["author", object.spec.artifact] : ["other"])[0]]`},
	} {
		test.Run(testCase.expression, func(test *testing.T) {
			expression, issues := environment.Compile(testCase.expression)
			require.NoError(test, issues.Err())
			require.Equal(test, testCase.paths, ImageResourcePaths(expression))
		})
	}
	require.Nil(test, ImageResourcePaths(nil))
	unchecked, issues := environment.Parse(`[object.spec.artifact]`)
	require.NoError(test, issues.Err())
	require.Nil(test, ImageResourcePaths(unchecked))
}
