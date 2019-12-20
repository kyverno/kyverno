package webhooks

import (
	"testing"

	"github.com/nirmata/kyverno/pkg/engine/response"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newPolicyResponse(policy, rule string, patchesStr []string, success bool) response.PolicyResponse {
	var patches [][]byte
	for _, p := range patchesStr {
		patches = append(patches, []byte(p))
	}

	return response.PolicyResponse{
		Policy: policy,
		Rules: []response.RuleResponse{
			response.RuleResponse{
				Name:    rule,
				Patches: patches,
				Success: success},
		},
	}
}

func newEngineResponse(policy, rule string, patchesStr []string, success bool, annotation map[string]string) response.EngineResponse {
	return response.EngineResponse{
		PatchedResource: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotation": annotation,
				},
			},
		},
		PolicyResponse: newPolicyResponse(policy, rule, patchesStr, success),
	}
}

func Test_empty_annotation(t *testing.T) {
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, true, nil)

	annPatches := generateAnnotationPatches([]response.EngineResponse{engineResponse})
	expectedPatches := `{"op":"add","path":"/metadata/annotations","value":{"policies.kyverno.patches":"[{\"policyname\":\"mutate-container\",\"patches\":[{\"rulename\":\"default-imagepullpolicy\",\"op\":\"replace\",\"path\":\"/spec/containers/0/imagePullPolicy\"}]}]"}}`
	assert.Assert(t, string(annPatches) == expectedPatches)
}

func Test_exist_annotation(t *testing.T) {
	annotation := map[string]string{
		"test": "annotation",
	}

	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, true, annotation)
	annPatches := generateAnnotationPatches([]response.EngineResponse{engineResponse})

	expectedPatches := `{"op":"add","path":"/metadata/annotations","value":{"policies.kyverno.patches":"[{\"policyname\":\"mutate-container\",\"patches\":[{\"rulename\":\"default-imagepullpolicy\",\"op\":\"replace\",\"path\":\"/spec/containers/0/imagePullPolicy\"}]}]"}}`
	assert.Assert(t, string(annPatches) == expectedPatches)
}

func Test_exist_kyverno_annotation(t *testing.T) {
	annotation := map[string]string{
		"policies.kyverno.patches": "old-annotation",
	}

	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, true, annotation)
	annPatches := generateAnnotationPatches([]response.EngineResponse{engineResponse})

	expectedPatches := `{"op":"add","path":"/metadata/annotations","value":{"policies.kyverno.patches":"[{\"policyname\":\"mutate-container\",\"patches\":[{\"rulename\":\"default-imagepullpolicy\",\"op\":\"replace\",\"path\":\"/spec/containers/0/imagePullPolicy\"}]}]"}}`
	assert.Assert(t, string(annPatches) == expectedPatches)
}

func Test_annotation_nil_patch(t *testing.T) {
	annotation := map[string]string{
		"policies.kyverno.patches": "old-annotation",
	}

	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", nil, true, annotation)
	annPatches := generateAnnotationPatches([]response.EngineResponse{engineResponse})
	assert.Assert(t, annPatches == nil)

	engineResponseNew := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{""}, true, annotation)
	annPatchesNew := generateAnnotationPatches([]response.EngineResponse{engineResponseNew})
	assert.Assert(t, annPatchesNew == nil)
}

func Test_annotation_failed_Patch(t *testing.T) {
	annotation := map[string]string{
		"policies.kyverno.patches": "old-annotation",
	}

	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", nil, false, annotation)
	annPatches := generateAnnotationPatches([]response.EngineResponse{engineResponse})

	assert.Assert(t, annPatches == nil)
}
