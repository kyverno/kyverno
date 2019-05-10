package policycontroller

import (
	"testing"

	"gotest.tools/assert"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyCopyFrom_Validate(t *testing.T) {
	copyFrom := types.PolicyCopyFrom{}
	assert.Assert(t, copyFrom.Validate() != nil)
	copyFrom.Name = "name"
	assert.Assert(t, copyFrom.Validate() != nil)
	copyFrom.Namespace = "ns"
	assert.Assert(t, copyFrom.Validate() == nil)
}

func TestPolicyConfigGenerator_Validate(t *testing.T) {
	// Not valid
	generator := types.PolicyConfigGenerator{}
	assert.Assert(t, generator.Validate() != nil)
	generator.Name = "generator-name"
	assert.Assert(t, generator.Validate() != nil)
	generator.Data = make(map[string]string)
	assert.Assert(t, generator.Validate() != nil)
	// Valid
	generator.Data["field"] = "value"
	assert.Assert(t, generator.Validate() == nil)
	generator.CopyFrom = &types.PolicyCopyFrom{
		Name:      "config-map-name",
		Namespace: "custom-ns",
	}
	assert.Assert(t, generator.Validate() == nil)
	generator.Data = nil
	assert.Assert(t, generator.Validate() == nil)
	// Not valid again
	generator.CopyFrom = nil
}

func TestPolicyPatch_Validate(t *testing.T) {
	// Not valid
	patch := types.PolicyPatch{}
	assert.Assert(t, patch.Validate() != nil)
	patch.Path = "/path"
	assert.Assert(t, patch.Validate() != nil)
	patch.Operation = "add"
	assert.Assert(t, patch.Validate() != nil)
	// Valid
	patch.Value = "some-value"
	assert.Assert(t, patch.Validate() == nil)
	patch.Operation = "replace"
	assert.Assert(t, patch.Validate() == nil)
	patch.Operation = "remove"
	assert.Assert(t, patch.Validate() == nil)
	// Valid without a value
	patch.Value = ""
	assert.Assert(t, patch.Validate() == nil)
	// Not valid again
	patch.Operation = "unknown"
	assert.Assert(t, patch.Validate() != nil)
	patch.Value = "some-another-value"
	assert.Assert(t, patch.Validate() != nil)
}

func TestPolicyResource_Validate_Name(t *testing.T) {
	// Not valid
	resource := types.PolicyResource{}
	assert.Assert(t, resource.Validate() != nil)
	resource.Kind = "Deployment"
	assert.Assert(t, resource.Validate() != nil)
	// Valid
	resourceName := "nginx"
	resource.Name = &resourceName
	assert.Assert(t, resource.Validate() == nil)
}

func TestPolicyResource_Validate_Selector(t *testing.T) {
	// Not valid
	resource := types.PolicyResource{
		Kind:     "ConfigMap",
		Selector: new(metav1.LabelSelector),
	}
	assert.Assert(t, resource.Validate() != nil)
	resource.Selector.MatchLabels = make(map[string]string)
	assert.Assert(t, resource.Validate() != nil)
	// Valid
	resource.Selector.MatchLabels["new-label"] = "new-value"
	assert.Assert(t, resource.Validate() == nil)
}

func makeValidRuleResource() types.PolicyResource {
	resourceName := "test-deployment"
	return types.PolicyResource{
		Kind: "Deployment",
		Name: &resourceName,
	}
}

func TestPolicyRule_Validate_Resource(t *testing.T) {
	// Not valid
	rule := types.PolicyRule{}
	assert.Assert(t, rule.Validate() != nil)
	// Empty
	rule.Resource = makeValidRuleResource()
	// Validate resource toi ensure that it is the only valid field
	assert.Assert(t, rule.Resource.Validate() == nil)
	assert.Assert(t, rule.Validate() != nil)
}

func TestPolicyRule_Validate_Patches(t *testing.T) {
	rule := types.PolicyRule{
		Resource: makeValidRuleResource(),
	}
	// Not empty, but not valid
	patch := types.PolicyPatch{}
	rule.Patches = append(rule.Patches, patch)
	// Not empty and valid
	assert.Assert(t, rule.Validate() != nil)
	rule.Patches[0] = types.PolicyPatch{
		Path:      "/",
		Operation: "add",
		Value:     "some",
	}
	assert.Assert(t, rule.Validate() == nil)
}

func TestPolicyRule_Validate_ConfigGenerators(t *testing.T) {
	rule := types.PolicyRule{
		Resource: makeValidRuleResource(),
	}
	// Not empty, but not valid
	rule.ConfigMapGenerator = &types.PolicyConfigGenerator{
		Name: "test-generator",
	}
	assert.Assert(t, rule.Validate() != nil)
	// Not empty and valid
	rule.ConfigMapGenerator.Data = make(map[string]string)
	rule.ConfigMapGenerator.Data["some-data"] = "some-value"
	assert.Assert(t, rule.Validate() == nil)
	rule.SecretGenerator = rule.ConfigMapGenerator
	assert.Assert(t, rule.Validate() == nil)
	rule.ConfigMapGenerator = nil
	assert.Assert(t, rule.Validate() == nil)
	// Not valid again
	rule.SecretGenerator.Name = ""
	assert.Assert(t, rule.Validate() != nil)
}
