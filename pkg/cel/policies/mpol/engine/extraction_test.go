package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

// jobSetWithReplicatedJobs builds a minimal JobSet-shaped object with one
// replicatedJob per image, matching the shape pkg/cel/autogen resolves for
// the "jobsets.v1alpha2.jobset.x-k8s.io" controller.
func jobSetWithReplicatedJobs(images ...string) *unstructured.Unstructured {
	jobs := make([]any, 0, len(images))
	for i, image := range images {
		jobs = append(jobs, map[string]any{
			"name": fmt.Sprintf("job-%d", i),
			"template": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{"name": "worker", "image": image},
							},
						},
					},
				},
			},
		})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "test-jobset", "namespace": "default"},
		"spec":       map[string]any{"replicatedJobs": jobs},
	}}
}

// buildExtractionModeMutatingPolicy builds a Pod-targeted MutatingPolicy,
// left completely unmodified, with autogen configured for a custom CRD via
// ExtractionReplacementsRef (see pkg/cel/policies/mpol/autogen).
func buildExtractionModeMutatingPolicy(expression string) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "inject-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: expression,
					},
				},
			},
		},
	}
}

func jobSetEngineRequest(obj *unstructured.Unstructured) celengine.EngineRequest {
	return celengine.Request(
		nil,
		schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
		schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
		"",
		obj.GetName(),
		obj.GetNamespace(),
		admissionv1.Create,
		authenticationv1.UserInfo{},
		obj,
		nil,
		false,
		nil,
	)
}

// firedPolicies filters resp.Policies down to entries that actually
// evaluated a rule, mirroring the equivalent vpol test helper: an
// extraction-mode autogen target must be the only one to fire against a
// JobSet, not the base Pod-targeted variant of the same policy.
func firedPolicies(resp EngineResponse) []MutatingPolicyResponse {
	var fired []MutatingPolicyResponse
	for _, p := range resp.Policies {
		if len(p.Rules) > 0 {
			fired = append(fired, p)
		}
	}
	return fired
}

func podTemplateAt(t *testing.T, root map[string]any, jobIndex int) map[string]any {
	t.Helper()
	jobs, ok, err := unstructured.NestedSlice(root, "spec", "replicatedJobs")
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, len(jobs), jobIndex)
	job, ok := jobs[jobIndex].(map[string]any)
	require.True(t, ok)
	tpl, ok, err := unstructured.NestedMap(job, "template", "spec", "template")
	require.NoError(t, err)
	require.True(t, ok)
	return tpl
}

func TestHandle_ExtractionMode_JobSet_Mutation(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(`Object{metadata: Object.metadata{labels: {"injected": "true"}}}`)
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := NewEngine(provider, noopNsResolver, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})

	obj := jobSetWithReplicatedJobs("bash:1.0")
	resp, err := eng.Handle(context.Background(), jobSetEngineRequest(obj), nil)
	require.NoError(t, err)

	fired := firedPolicies(resp)
	require.Len(t, fired, 1, "exactly one policy entry (the extraction-mode JobSet target) should have fired, not the base Pod-targeted one")
	require.Len(t, fired[0].Rules, 1)
	assert.Equal(t, engineapi.RuleStatusPass, fired[0].Rules[0].Status())

	require.NotNil(t, resp.PatchedResource)
	tpl := podTemplateAt(t, resp.PatchedResource.Object, 0)
	labels, ok, err := unstructured.NestedStringMap(tpl, "metadata", "labels")
	require.NoError(t, err)
	require.True(t, ok, "mutation should have added a label to the pod template")
	assert.Equal(t, "true", labels["injected"])

	// The real JSON patch is computed against the whole JobSet, at the real
	// nested path - not a synthetic top-level Pod patch.
	er := EngineResponse{Resource: resp.Resource, PatchedResource: resp.PatchedResource}
	patches := er.GetPatches()
	require.NotEmpty(t, patches)
	var sawTemplatePath bool
	for _, p := range patches {
		if p.Path == "/spec/replicatedJobs/0/template/spec/template/metadata" {
			sawTemplatePath = true
		}
	}
	assert.True(t, sawTemplatePath, "expected a patch operation at the pod template's real path, got: %+v", patches)

	// The original object backing resp.Resource must be untouched.
	originalTpl := podTemplateAt(t, resp.Resource.Object, 0)
	_, hadMetadata := originalTpl["metadata"]
	assert.False(t, hadMetadata, "original object must not be mutated in place")
}

func TestHandle_ExtractionMode_JobSet_MultipleTemplatesMutatedIndependently(t *testing.T) {
	// The mutation's label value is derived from each synthesized Pod's own
	// image, so if templates were mutated independently (rather than one
	// evaluation's result leaking into every template) each replicatedJob
	// ends up with a different label value.
	policy := buildExtractionModeMutatingPolicy(`Object{metadata: Object.metadata{labels: {"image": object.spec.containers[0].image}}}`)
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := NewEngine(provider, noopNsResolver, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})

	obj := jobSetWithReplicatedJobs("bash:1.0", "bash:2.0")
	resp, err := eng.Handle(context.Background(), jobSetEngineRequest(obj), nil)
	require.NoError(t, err)

	fired := firedPolicies(resp)
	require.Len(t, fired, 1)
	assert.Equal(t, engineapi.RuleStatusPass, fired[0].Rules[0].Status())

	require.NotNil(t, resp.PatchedResource)
	tpl0 := podTemplateAt(t, resp.PatchedResource.Object, 0)
	tpl1 := podTemplateAt(t, resp.PatchedResource.Object, 1)

	labels0, ok, err := unstructured.NestedStringMap(tpl0, "metadata", "labels")
	require.NoError(t, err)
	require.True(t, ok)
	labels1, ok, err := unstructured.NestedStringMap(tpl1, "metadata", "labels")
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, "bash:1.0", labels0["image"])
	assert.Equal(t, "bash:2.0", labels1["image"])
}

func TestHandle_ExtractionMode_JobSet_NoPodTemplateFound(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(`Object{metadata: Object.metadata{labels: {"injected": "true"}}}`)
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := NewEngine(provider, noopNsResolver, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})

	// no replicatedJobs at all - nothing shaped like a pod template exists.
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "empty-jobset", "namespace": "default"},
		"spec":       map[string]any{"replicatedJobs": []any{}},
	}}
	resp, err := eng.Handle(context.Background(), jobSetEngineRequest(obj), nil)
	require.NoError(t, err)

	fired := firedPolicies(resp)
	require.Len(t, fired, 1)
	require.Len(t, fired[0].Rules, 1)
	rule := fired[0].Rules[0]
	assert.Equal(t, engineapi.RuleStatusError, rule.Status())
	assert.Contains(t, rule.Message(), "no pod template found")
	assert.Nil(t, resp.PatchedResource)
}

func TestHandle_ExtractionMode_JobSet_MatchConditionsSkipAllTemplates(t *testing.T) {
	policy := buildExtractionModeMutatingPolicy(`Object{metadata: Object.metadata{labels: {"injected": "true"}}}`)
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{
		{Name: "never-matches", Expression: `object.spec.containers[0].image == "this-image-never-appears"`},
	}
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := NewEngine(provider, noopNsResolver, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})

	obj := jobSetWithReplicatedJobs("bash:1.0")
	resp, err := eng.Handle(context.Background(), jobSetEngineRequest(obj), nil)
	require.NoError(t, err)

	fired := firedPolicies(resp)
	require.Len(t, fired, 1)
	require.Len(t, fired[0].Rules, 1)
	assert.Equal(t, engineapi.RuleStatusSkip, fired[0].Rules[0].Status())
	assert.Nil(t, resp.PatchedResource)
}

// TestEvaluateExtracted_NoObjectToMutate documents that ExtractionMode
// mutation only ever applies to the object being admitted: a request with
// no new object (e.g. a real DELETE, where the resource only lives in
// oldObject) is skipped rather than synthesized against oldObject, unlike
// vpol's read-only validation counterpart.
func TestEvaluateExtracted_NoObjectToMutate(t *testing.T) {
	e := &engineImpl{typeConverter: &fakeTypeConverter{}, contextProvider: &libs.FakeContextProvider{}}
	oldObj := jobSetWithReplicatedJobs("bash:1.0")
	attr := admission.NewAttributesRecord(
		nil, oldObj,
		schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
		"default", "test-jobset",
		schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
		"", admission.Delete, nil, false, nil,
	)

	result := e.evaluateExtracted(context.Background(), Policy{}, attr, admissionv1.AdmissionRequest{}, nil, false)
	assert.Nil(t, result)
}
