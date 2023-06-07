package mutate

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/go-logr/logr"
	types "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func loadYaml(t *testing.T, file string) []byte {
	bytes, err := os.ReadFile(file)
	require.NoError(t, err)
	yaml, err := yaml.YAMLToJSON(bytes)
	require.NoError(t, err)
	return yaml
}

// jsonPatch is used to build test patches
type jsonPatch struct {
	Path      string             `json:"path,omitempty" yaml:"path,omitempty"`
	Operation string             `json:"op,omitempty" yaml:"op,omitempty"`
	Value     apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

func applyPatches(rule *types.Rule, resource unstructured.Unstructured) (*engineapi.RuleResponse, unstructured.Unstructured) {
	mutateResp := Mutate(rule, context.NewContext(jmespath.New(config.NewDefaultConfiguration(false))), resource, logr.Discard())
	if mutateResp.Status != engineapi.RuleStatusPass {
		return engineapi.NewRuleResponse("", engineapi.Mutation, mutateResp.Message, mutateResp.Status), resource
	}
	return engineapi.RulePass("", engineapi.Mutation, mutateResp.Message), mutateResp.PatchedResource
}

func TestProcessPatches_EmptyPatches(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	rule := types.Rule{Name: "emptyRule"}

	// apply patches
	rr, patched := applyPatches(&rule, resource)

	// assert
	require.NotNil(t, rr)
	require.Equal(t, engineapi.RuleStatusError, rr.Status())
	require.Equal(t, resource, patched)
}

func makeAddIsMutatedLabelPatch() jsonPatch {
	return jsonPatch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
}

func makeRuleWithPatch(t *testing.T, patch jsonPatch) *types.Rule {
	patches := []jsonPatch{patch}
	return makeRuleWithPatches(t, patches)
}

func makeRuleWithPatches(t *testing.T, patches []jsonPatch) *types.Rule {
	jsonPatches, err := json.Marshal(patches)
	if err != nil {
		t.Errorf("failed to marshal patch: %v", err)
	}

	mutation := types.Mutation{
		PatchesJSON6902: string(jsonPatches),
	}
	return &types.Rule{
		Mutation: mutation,
	}
}

func TestProcessPatches_EmptyDocument(t *testing.T) {
	// load resource
	var resource unstructured.Unstructured

	// use rule
	rule := makeRuleWithPatch(t, makeAddIsMutatedLabelPatch())

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusError, rr.Status())
	require.Equal(t, resource, patched)
}

func TestProcessPatches_AllEmpty(t *testing.T) {
	// load resource
	var resource unstructured.Unstructured

	// use rule
	rule := types.Rule{}

	// apply patches
	rr, patched := applyPatches(&rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusError, rr.Status())
	require.Equal(t, resource, patched)
}

func TestProcessPatches_AddPathDoesntExist(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	patch := makeAddIsMutatedLabelPatch()
	patch.Path = "/metadata/additional/is-mutated"
	rule := makeRuleWithPatch(t, patch)

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusPass, rr.Status())
	require.NotEqual(t, patched.UnstructuredContent(), resource.UnstructuredContent())
	unstructured.SetNestedField(resource.UnstructuredContent(), "true", "metadata", "additional", "is-mutated")
	require.Equal(t, resource, patched)
}

func TestProcessPatches_RemovePathDoesntExist(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	patch := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	rule := makeRuleWithPatch(t, patch)

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusSkip, rr.Status())
	require.Equal(t, resource, patched)
}

func TestProcessPatches_AddAndRemovePathsDontExist_EmptyResult(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	patch1 := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := jsonPatch{Path: "/spec/labels/label3", Operation: "add", Value: "label3Value"}
	rule := makeRuleWithPatches(t, []jsonPatch{patch1, patch2})

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusPass, rr.Status())
	require.NotEqual(t, patched.UnstructuredContent(), resource.UnstructuredContent())
	unstructured.SetNestedField(resource.UnstructuredContent(), "label3Value", "spec", "labels", "label3")
	require.Equal(t, resource, patched)
}

func TestProcessPatches_AddAndRemovePathsDontExist_ContinueOnError_NotEmptyResult(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	patch1 := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := jsonPatch{Path: "/spec/labels/label2", Operation: "remove", Value: "label2Value"}
	patch3 := jsonPatch{Path: "/metadata/labels/label3", Operation: "add", Value: "label3Value"}
	rule := makeRuleWithPatches(t, []jsonPatch{patch1, patch2, patch3})

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusPass, rr.Status())
	require.NotEqual(t, patched.UnstructuredContent(), resource.UnstructuredContent())
	unstructured.SetNestedField(resource.UnstructuredContent(), "label3Value", "metadata", "labels", "label3")
	require.Equal(t, resource, patched)
}

func TestProcessPatches_RemovePathDoesntExist_EmptyResult(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	patch := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	rule := makeRuleWithPatch(t, patch)

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusSkip, rr.Status())
	require.Equal(t, resource, patched)
}

func TestProcessPatches_RemovePathDoesntExist_NotEmptyResult(t *testing.T) {
	// load resource
	bytes := loadYaml(t, "testdata/endpoints.yaml")
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))

	// use rule
	patch1 := jsonPatch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := jsonPatch{Path: "/metadata/labels/label2", Operation: "add", Value: "label2Value"}
	rule := makeRuleWithPatches(t, []jsonPatch{patch1, patch2})

	// apply patches
	rr, patched := applyPatches(rule, resource)

	// assert
	require.Equal(t, engineapi.RuleStatusPass, rr.Status())
	require.NotEqual(t, patched.UnstructuredContent(), resource.UnstructuredContent())
	unstructured.SetNestedField(resource.UnstructuredContent(), "label2Value", "metadata", "labels", "label2")
	require.Equal(t, resource, patched)
}
