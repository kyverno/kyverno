package autogen

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const podLabelsMutation = `Object{metadata: Object.metadata{labels: {'checked': 'true'}}}`

// TestAutogenBareBuiltInConvertibleApplyConfiguration verifies that a convertible
// ApplyConfiguration on a bare built-in controller (deployments) is correctly
// wrapped for the pod template under AutogenDefaults, rewriting variables and
// match conditions as selectors.
func TestAutogenBareBuiltInConvertibleApplyConfiguration(t *testing.T) {
	policy := newMutatingPolicyForController([]string{"deployments"}, []admissionregistrationv1alpha1.Mutation{
		{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{spec: Object.spec{containers: []}}`}},
	})
	policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "podLabels", Expression: podLabelsMutation}}
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "check", Expression: `object.spec.containers.size() > 0`}}
	inputCopy := policy.DeepCopy()

	result, err := Autogen(policy)
	require.NoError(t, err)
	require.Contains(t, result, autogen.AutogenDefaults, "got refs %v", sets.List(sets.KeySet(result)))
	assert.NotContains(t, result, ExtractionReplacementsRef, "convertible mutation must stay under AutogenDefaults")

	generated := result[autogen.AutogenDefaults]
	assert.Equal(t, []policiesv1beta1.Target{{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}}, generated.Targets)

	require.Len(t, generated.Spec.Mutations, 1)
	assert.Contains(t, generated.Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.template.spec{")

	require.Len(t, generated.Spec.Variables, 1)
	assert.Contains(t, generated.Spec.Variables[0].Expression, "Object.spec.template.metadata{")

	require.Len(t, generated.Spec.MatchConditions, 1)
	assert.Equal(t, `object.spec.template.spec.containers.size() > 0`, generated.Spec.MatchConditions[0].Expression)

	assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
}

// When a bare built-in controller encounters an unconvertible ApplyConfiguration
// mutation, it must fall back to ExtractionReplacementsRef. Emitting an
// unrewritten Pod-shaped mutation or match condition under AutogenDefaults or
// AutogenCronjobs would evaluate directly against the workload root and fail
// admission. The conversion is atomic: variables, match conditions, and mutations
// are rolled back to the original spec without partial rewriting.
func TestAutogenBareBuiltInFallsBackToExtractionOnUnconvertibleMutation(t *testing.T) {
	for _, tt := range []struct {
		name       string
		controller string
		target     policiesv1beta1.Target
		mutations  []admissionregistrationv1alpha1.Mutation
	}{
		{
			name:       "deployments: unsupported mutation first",
			controller: "deployments",
			target:     policiesv1beta1.Target{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
			mutations: []admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}},
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: podLabelsMutation}},
			},
		},
		{
			name:       "deployments: unsupported mutation last (atomic rollback)",
			controller: "deployments",
			target:     policiesv1beta1.Target{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
			mutations: []admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: podLabelsMutation}},
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}},
			},
		},
		{
			name:       "cronjobs: unsupported mutation first",
			controller: "cronjobs",
			target:     policiesv1beta1.Target{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"},
			mutations: []admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}},
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: podLabelsMutation}},
			},
		},
		{
			name:       "cronjobs: unsupported mutation last (atomic rollback)",
			controller: "cronjobs",
			target:     policiesv1beta1.Target{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"},
			mutations: []admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: podLabelsMutation}},
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			policy := newMutatingPolicyForController([]string{tt.controller}, tt.mutations)
			policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "podPatch", Expression: `Object{spec: Object.spec{containers: []}}`}}
			policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "check", Expression: `object.spec.containers.size() > 0`}}
			inputCopy := policy.DeepCopy()

			result, err := Autogen(policy)
			require.NoError(t, err)
			require.Contains(t, result, autogen.ExtractionReplacementsRef, "got refs %v", sets.List(sets.KeySet(result)))
			assert.NotContains(t, result, autogen.AutogenDefaults, "no raw Pod-shaped mutation or condition under AutogenDefaults")
			assert.NotContains(t, result, autogen.AutogenCronjobs, "no raw Pod-shaped mutation or condition under AutogenCronjobs")

			generated := result[autogen.ExtractionReplacementsRef]
			assert.Equal(t, []policiesv1beta1.Target{tt.target}, generated.Targets)

			// No partially converted spec: every expression is the original,
			// Pod-shaped one, and nothing was rewritten to the template.
			require.Len(t, generated.Spec.Variables, 1)
			assert.Equal(t, `Object{spec: Object.spec{containers: []}}`, generated.Spec.Variables[0].Expression)

			require.Len(t, generated.Spec.MatchConditions, 1)
			assert.Equal(t, `object.spec.containers.size() > 0`, generated.Spec.MatchConditions[0].Expression)

			require.Len(t, generated.Spec.Mutations, len(tt.mutations))
			for i, mutation := range tt.mutations {
				assert.Equal(t, mutation.ApplyConfiguration.Expression, generated.Spec.Mutations[i].ApplyConfiguration.Expression)
				assert.NotContains(t, generated.Spec.Mutations[i].ApplyConfiguration.Expression, "spec.template")
				assert.NotContains(t, generated.Spec.Mutations[i].ApplyConfiguration.Expression, "spec.jobTemplate")
			}
			assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
		})
	}
}

// JSONPatch is not rewritten for the pod template. A bare built-in name keeps
// its existing behaviour: the target stays built-in and the patch expression is
// passed through unchanged instead of the policy moving to extraction mode.
func TestAutogenBareBuiltInJSONPatchKeepsBuiltInTarget(t *testing.T) {
	expression := `[JSONPatch{op: "add", path: "/metadata/labels/checked", value: "true"}]`
	policy := newMutatingPolicyForController([]string{"deployments"}, []admissionregistrationv1alpha1.Mutation{
		{JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: expression}},
	})

	result, err := Autogen(policy)
	require.NoError(t, err)
	require.Contains(t, result, autogen.AutogenDefaults, "got refs %v", sets.List(sets.KeySet(result)))
	assert.NotContains(t, result, ExtractionReplacementsRef)

	generated := result[autogen.AutogenDefaults]
	require.Len(t, generated.Spec.Mutations, 1)
	require.NotNil(t, generated.Spec.Mutations[0].JSONPatch)
	assert.Equal(t, expression, generated.Spec.Mutations[0].JSONPatch.Expression)
}

// A supported pod-shaped mutation next to an unconverted JSONPatch keeps the
// bare target in built-in mode: the ApplyConfiguration is wrapped for the pod
// template, the JSONPatch is passed through, and the policy never moves to
// extraction mode. Variables and match conditions are rewritten as selectors
// for the template.
func TestAutogenBareBuiltInMixedMutationsStayConverted(t *testing.T) {
	jsonPatch := `[JSONPatch{op: "add", path: "/metadata/labels/checked", value: "true"}]`
	policy := newMutatingPolicyForController([]string{"deployments"}, []admissionregistrationv1alpha1.Mutation{
		{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{spec: Object.spec{containers: []}}`}},
		{JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: jsonPatch}},
	})
	policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "podPatch", Expression: podLabelsMutation}}
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "check", Expression: `object.spec.containers.size() > 0`}}
	inputCopy := policy.DeepCopy()

	result, err := Autogen(policy)
	require.NoError(t, err)
	require.Contains(t, result, autogen.AutogenDefaults, "got refs %v", sets.List(sets.KeySet(result)))
	assert.NotContains(t, result, ExtractionReplacementsRef)

	generated := result[autogen.AutogenDefaults]
	require.Len(t, generated.Spec.Mutations, 2)
	assert.Contains(t, generated.Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.template.spec{")
	require.NotNil(t, generated.Spec.Mutations[1].JSONPatch)
	assert.Equal(t, jsonPatch, generated.Spec.Mutations[1].JSONPatch.Expression)

	require.Len(t, generated.Spec.Variables, 1)
	assert.Contains(t, generated.Spec.Variables[0].Expression, "Object.spec.template.metadata{")
	require.Len(t, generated.Spec.MatchConditions, 1)
	assert.Equal(t, `object.spec.template.spec.containers.size() > 0`, generated.Spec.MatchConditions[0].Expression)

	assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
}

// Explicitly remapped built-in targets (e.g. deployments.v1.apps, cronjobs.v1.batch)
// use built-in pod template conversion for supported ApplyConfiguration mutations,
// but fall back to extraction mode for JSONPatch or unconvertible mutations.
func TestAutogenExplicitRemappedBuiltInTargets(t *testing.T) {
	t.Run("convertible ApplyConfiguration stays in built-in group", func(t *testing.T) {
		policy := newMutatingPolicyForController(
			[]string{"deployments.v1.apps"},
			[]admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{spec: Object.spec{containers: []}}`}},
			},
		)
		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Contains(t, result, autogen.AutogenDefaults)
		assert.NotContains(t, result, ExtractionReplacementsRef)
		assert.Contains(t, result[autogen.AutogenDefaults].Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.template.spec{")
	})

	t.Run("unconvertible ApplyConfiguration falls back to extraction", func(t *testing.T) {
		policy := newMutatingPolicyForController(
			[]string{"deployments.v1.apps"},
			[]admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}},
			},
		)
		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Contains(t, result, ExtractionReplacementsRef)
		assert.NotContains(t, result, autogen.AutogenDefaults)
		assert.Equal(t, `variables.podPatch`, result[ExtractionReplacementsRef].Spec.Mutations[0].ApplyConfiguration.Expression)
	})

	t.Run("JSONPatch falls back to extraction for remapped built-in", func(t *testing.T) {
		policy := newMutatingPolicyForController(
			[]string{"deployments.v1.apps"},
			[]admissionregistrationv1alpha1.Mutation{
				{JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: `[JSONPatch{op: "add", path: "/metadata/labels/checked", value: "true"}]`}},
			},
		)
		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Contains(t, result, ExtractionReplacementsRef)
		assert.NotContains(t, result, autogen.AutogenDefaults)
	})
}

// Controllers that resolve to different replacement groups must each keep their
// own generated spec: the defaults and CronJob pod-template paths differ, and a
// custom workload CRD keeps its extraction rule unwrapped.
func TestAutogenMixedControllerGroupsKeepTheirOwnRules(t *testing.T) {
	policy := newMutatingPolicyForController(
		[]string{"deployments.v1.apps", "cronjobs.v1.batch", "jobsets.v1alpha2.jobset.x-k8s.io"},
		[]admissionregistrationv1alpha1.Mutation{
			{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `Object{spec: Object.spec{containers: []}}`}},
		},
	)
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "check", Expression: `object.spec.containers.size() > 0`}}

	result, err := Autogen(policy)
	require.NoError(t, err)
	require.Len(t, result, 3, "got refs %v", sets.List(sets.KeySet(result)))

	defaults := result[autogen.AutogenDefaults]
	assert.Equal(t, []policiesv1beta1.Target{{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}}, defaults.Targets)
	assert.Contains(t, defaults.Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.template.spec{")
	assert.Equal(t, `object.spec.template.spec.containers.size() > 0`, defaults.Spec.MatchConditions[0].Expression)

	cronjobs := result[autogen.AutogenCronjobs]
	assert.Equal(t, []policiesv1beta1.Target{{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"}}, cronjobs.Targets)
	assert.Contains(t, cronjobs.Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.jobTemplate.spec.template.spec{")

	extract := result[ExtractionReplacementsRef]
	assert.Equal(t, []policiesv1beta1.Target{{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets", Kind: "Jobset"}}, extract.Targets)
	assert.Equal(t, `Object{spec: Object.spec{containers: []}}`, extract.Spec.Mutations[0].ApplyConfiguration.Expression)
	assert.Equal(t, `object.spec.containers.size() > 0`, extract.Spec.MatchConditions[0].Expression)

	// Each rule matches only its own controller.
	for _, tt := range []struct {
		spec     *policiesv1beta1.MutatingPolicySpec
		group    string
		resource string
	}{
		{defaults.Spec, "apps", "deployments"},
		{cronjobs.Spec, "batch", "cronjobs"},
		{extract.Spec, "jobset.x-k8s.io", "jobsets"},
	} {
		require.Len(t, tt.spec.MatchConstraints.ResourceRules, 1)
		rule := tt.spec.MatchConstraints.ResourceRules[0]
		assert.Equal(t, []string{tt.group}, rule.APIGroups)
		assert.Equal(t, []string{tt.resource}, rule.Resources)
	}
}

// A failed conversion falls back to extraction mode without affecting sibling
// rules that convert successfully, and leaves the input policy intact.
func TestAutogenFailedConversionLeavesSiblingRulesAlone(t *testing.T) {
	t.Run("all unconvertible controllers fall back together to extraction", func(t *testing.T) {
		policy := newMutatingPolicyForController(
			[]string{"deployments.v1.apps", "cronjobs.v1.batch"},
			[]admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: `variables.podPatch`}},
			},
		)
		inputCopy := policy.DeepCopy()

		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Len(t, result, 1, "got refs %v", sets.List(sets.KeySet(result)))

		extract := result[ExtractionReplacementsRef]
		assert.Equal(t, []policiesv1beta1.Target{
			{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"},
			{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"},
		}, extract.Targets)
		assert.Equal(t, `variables.podPatch`, extract.Spec.Mutations[0].ApplyConfiguration.Expression)
		assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
	})

	t.Run("one controller converts while sibling controller falls back", func(t *testing.T) {
		// Bare deployments supports JSONPatch and converts ApplyConfiguration to template.
		// Explicit cronjobs.v1.batch falls back to extraction when JSONPatch is present.
		jsonPatchExpr := `[JSONPatch{op: "add", path: "/metadata/labels/checked", value: "true"}]`
		applyConfigExpr := `Object{spec: Object.spec{containers: []}}`
		policy := newMutatingPolicyForController(
			[]string{"deployments", "cronjobs.v1.batch"},
			[]admissionregistrationv1alpha1.Mutation{
				{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: applyConfigExpr}},
				{JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: jsonPatchExpr}},
			},
		)
		policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "check", Expression: `object.spec.containers.size() > 0`}}
		inputCopy := policy.DeepCopy()

		result, err := Autogen(policy)
		require.NoError(t, err)
		require.Len(t, result, 2, "got refs %v", sets.List(sets.KeySet(result)))

		// Deployments converted successfully under AutogenDefaults
		require.Contains(t, result, autogen.AutogenDefaults)
		defaults := result[autogen.AutogenDefaults]
		assert.Equal(t, []policiesv1beta1.Target{{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}}, defaults.Targets)
		require.Len(t, defaults.Spec.Mutations, 2)
		assert.Contains(t, defaults.Spec.Mutations[0].ApplyConfiguration.Expression, "Object.spec.template.spec{")
		require.NotNil(t, defaults.Spec.Mutations[1].JSONPatch)
		assert.Equal(t, jsonPatchExpr, defaults.Spec.Mutations[1].JSONPatch.Expression)
		require.Len(t, defaults.Spec.MatchConditions, 1)
		assert.Equal(t, `object.spec.template.spec.containers.size() > 0`, defaults.Spec.MatchConditions[0].Expression)

		// CronJobs fell back to ExtractionReplacementsRef with clean unmutated spec
		require.Contains(t, result, ExtractionReplacementsRef)
		extract := result[ExtractionReplacementsRef]
		assert.Equal(t, []policiesv1beta1.Target{{Group: "batch", Version: "v1", Resource: "cronjobs", Kind: "CronJob"}}, extract.Targets)
		require.Len(t, extract.Spec.Mutations, 2)
		assert.Equal(t, applyConfigExpr, extract.Spec.Mutations[0].ApplyConfiguration.Expression)
		assert.NotContains(t, extract.Spec.Mutations[0].ApplyConfiguration.Expression, "spec.template")
		assert.NotContains(t, extract.Spec.Mutations[0].ApplyConfiguration.Expression, "spec.jobTemplate")
		require.NotNil(t, extract.Spec.Mutations[1].JSONPatch)
		assert.Equal(t, jsonPatchExpr, extract.Spec.Mutations[1].JSONPatch.Expression)
		require.Len(t, extract.Spec.MatchConditions, 1)
		assert.Equal(t, `object.spec.containers.size() > 0`, extract.Spec.MatchConditions[0].Expression)

		assert.Equal(t, inputCopy, policy, "input policy must be unchanged")
	})
}
