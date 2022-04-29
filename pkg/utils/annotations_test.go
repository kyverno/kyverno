package utils

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/response"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func newPolicyResponse(policy, rule string, patchesStr []string, status response.RuleStatus) response.PolicyResponse {
	var patches [][]byte
	for _, p := range patchesStr {
		patches = append(patches, []byte(p))
	}

	return response.PolicyResponse{
		Policy: response.PolicySpec{Name: policy},
		Rules: []response.RuleResponse{
			{
				Name:    rule,
				Patches: patches,
				Status:  status,
			},
		},
	}
}

func newEngineResponse(policy, rule string, patchesStr []string, status response.RuleStatus, annotation map[string]interface{}) *response.EngineResponse {
	return &response.EngineResponse{
		PatchedResource: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": annotation,
				},
			},
		},
		PolicyResponse: newPolicyResponse(policy, rule, patchesStr, status),
	}
}

func Test_empty_annotation(t *testing.T) {
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, response.RuleStatusPass, nil)
	annPatches := GenerateAnnotationPatches([]*response.EngineResponse{engineResponse}, log.Log)
	expectedPatches := `{"path":"/metadata/annotations","op":"add","value":{"policies.kyverno.io/last-applied-patches":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}}`
	assert.Equal(t, string(annPatches[0]), expectedPatches)
}

func Test_exist_annotation(t *testing.T) {
	annotation := map[string]interface{}{
		"test": "annotation",
	}
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, response.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]*response.EngineResponse{engineResponse}, log.Log)
	expectedPatches := `{"path":"/metadata/annotations/policies.kyverno.io~1last-applied-patches","op":"add","value":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}`
	assert.Equal(t, string(annPatches[0]), expectedPatches)
}

func Test_exist_kyverno_annotation(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.patches": "old-annotation",
	}
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, response.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]*response.EngineResponse{engineResponse}, log.Log)
	expectedPatches := `{"path":"/metadata/annotations/policies.kyverno.io~1last-applied-patches","op":"add","value":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}`
	assert.Equal(t, string(annPatches[0]), expectedPatches)
}

func Test_annotation_nil_patch(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.patches": "old-annotation",
	}
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", nil, response.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]*response.EngineResponse{engineResponse}, log.Log)
	assert.Assert(t, annPatches == nil)
	engineResponseNew := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{""}, response.RuleStatusPass, annotation)
	annPatchesNew := GenerateAnnotationPatches([]*response.EngineResponse{engineResponseNew}, log.Log)
	assert.Assert(t, annPatchesNew == nil)
}

func Test_annotation_failed_Patch(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.patches": "old-annotation",
	}
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", nil, response.RuleStatusFail, annotation)
	annPatches := GenerateAnnotationPatches([]*response.EngineResponse{engineResponse}, log.Log)
	assert.Assert(t, annPatches == nil)
}

func Test_exist_patches(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.io/patches": "present",
	}
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, response.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]*response.EngineResponse{engineResponse}, log.Log)
	expectedPatches1 := `{"path":"/metadata/annotations/policies.kyverno.io~1patches","op":"remove"}`
	expectedPatches2 := `{"path":"/metadata/annotations/policies.kyverno.io~1last-applied-patches","op":"add","value":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}`
	assert.Equal(t, string(annPatches[0]), expectedPatches1)
	assert.Equal(t, string(annPatches[1]), expectedPatches2)
}
