package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newPolicyResponse(rule string, patchesStr []string, status engineapi.RuleStatus) engineapi.PolicyResponse {
	var patches [][]byte
	for _, p := range patchesStr {
		patches = append(patches, []byte(p))
	}

	return engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{
			*engineapi.NewRuleResponse(rule, engineapi.Mutation, "", status).WithPatches(patches...),
		},
	}
}

func newEngineResponse(policy, rule string, patchesStr []string, status engineapi.RuleStatus, annotation map[string]interface{}) engineapi.EngineResponse {
	p := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policy,
		},
	}
	policyResponse := newPolicyResponse(rule, patchesStr, status)
	response := engineapi.NewEngineResponse(unstructured.Unstructured{}, p, nil).WithPolicyResponse(policyResponse)
	response.PatchedResource = unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": annotation,
			},
		},
	}
	return response
}

func Test_empty_annotation(t *testing.T) {
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, engineapi.RuleStatusPass, nil)
	annPatches := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponse}, logging.GlobalLogger())
	expectedPatches := `{"path":"/metadata/annotations","op":"add","value":{"policies.kyverno.io/last-applied-patches":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}}`
	assert.Equal(t, string(annPatches[0]), expectedPatches)
}

func Test_exist_annotation(t *testing.T) {
	annotation := map[string]interface{}{
		"test": "annotation",
	}
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, engineapi.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponse}, logging.GlobalLogger())
	expectedPatches := `{"path":"/metadata/annotations/policies.kyverno.io~1last-applied-patches","op":"add","value":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}`
	assert.Equal(t, string(annPatches[0]), expectedPatches)
}

func Test_exist_kyverno_annotation(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.patches": "old-annotation",
	}
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, engineapi.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponse}, logging.GlobalLogger())
	expectedPatches := `{"path":"/metadata/annotations/policies.kyverno.io~1last-applied-patches","op":"add","value":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}`
	assert.Equal(t, string(annPatches[0]), expectedPatches)
}

func Test_annotation_nil_patch(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.patches": "old-annotation",
	}
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", nil, engineapi.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponse}, logging.GlobalLogger())
	assert.Assert(t, annPatches == nil)
	engineResponseNew := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{""}, engineapi.RuleStatusPass, annotation)
	annPatchesNew := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponseNew}, logging.GlobalLogger())
	assert.Assert(t, annPatchesNew == nil)
}

func Test_annotation_failed_Patch(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.patches": "old-annotation",
	}
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", nil, engineapi.RuleStatusFail, annotation)
	annPatches := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponse}, logging.GlobalLogger())
	assert.Assert(t, annPatches == nil)
}

func Test_exist_patches(t *testing.T) {
	annotation := map[string]interface{}{
		"policies.kyverno.io/patches": "present",
	}
	patchStr := `{ "op": "replace", "path": "/spec/containers/0/imagePullPolicy", "value": "IfNotPresent" }`
	engineResponse := newEngineResponse("mutate-container", "default-imagepullpolicy", []string{patchStr}, engineapi.RuleStatusPass, annotation)
	annPatches := GenerateAnnotationPatches([]engineapi.EngineResponse{engineResponse}, logging.GlobalLogger())
	expectedPatches1 := `{"path":"/metadata/annotations/policies.kyverno.io~1patches","op":"remove"}`
	expectedPatches2 := `{"path":"/metadata/annotations/policies.kyverno.io~1last-applied-patches","op":"add","value":"default-imagepullpolicy.mutate-container.kyverno.io: replaced /spec/containers/0/imagePullPolicy\n"}`
	assert.Equal(t, string(annPatches[0]), expectedPatches1)
	assert.Equal(t, string(annPatches[1]), expectedPatches2)
}
