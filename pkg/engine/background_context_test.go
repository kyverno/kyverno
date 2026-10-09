package engine

import (
	"context"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestApplyBackgroundChecks_IsolatesRequestObjectAcrossRules reproduces issue #14526:
// after rule 1 temporarily swaps request.object to the old resource, rule 2 must still
// see distinct request.object and request.oldObject values.
func TestApplyBackgroundChecks_IsolatesRequestObjectAcrossRules(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	localJP := jmespath.New(cfg)
	e := NewEngine(cfg, localJP, nil, nil, imageverifycache.DisabledImageVerifyCache(),
		func(kyverno.PolicyInterface, kyverno.Rule) engineapi.ContextLoader {
			return loaderFunc(func(context.Context) error { return nil })
		}, nil, nil)

	target := []kyverno.TargetResourceSpec{{
		TargetSelector: kyverno.TargetSelector{
			ResourceSpec: kyverno.ResourceSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "dummy",
				Namespace:  "default",
			},
		},
	}}

	policy := &kyverno.ClusterPolicy{
		Spec: kyverno.Spec{
			Rules: []kyverno.Rule{
				{
					Name: "rule-1-name-mismatch",
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"Secret"},
						},
					},
					RawAnyAllConditions: &kyverno.ConditionsWrapper{
						Conditions: kyverno.AnyAllConditions{
							AllConditions: []kyverno.Condition{{
								RawKey:   kyverno.ToJSON("{{ request.object.metadata.name }}"),
								Operator: kyverno.ConditionOperators["Equals"],
								RawValue: kyverno.ToJSON("wrong-name"),
							}},
						},
					},
					Mutation: &kyverno.Mutation{
						Targets: target,
					},
				},
				{
					Name: "rule-2-data-changed",
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"Secret"},
						},
					},
					RawAnyAllConditions: &kyverno.ConditionsWrapper{
						Conditions: kyverno.AnyAllConditions{
							AllConditions: []kyverno.Condition{{
								RawKey:   kyverno.ToJSON("{{ request.object.data.foo }}"),
								Operator: kyverno.ConditionOperators["NotEquals"],
								RawValue: kyverno.ToJSON("{{ request.oldObject.data.foo }}"),
							}},
						},
					},
					Mutation: &kyverno.Mutation{
						Targets: target,
					},
				},
			},
		},
	}

	newSecret := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "test-secret",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"foo": "new-value",
		},
	}}
	oldSecret := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "test-secret",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"foo": "old-value",
		},
	}}

	pCtx, err := NewPolicyContext(localJP, newSecret, kyverno.Update, nil, cfg)
	require.NoError(t, err)
	pCtx = pCtx.WithOldResource(oldSecret)
	require.NoError(t, pCtx.JSONContext().AddOldResource(oldSecret.Object))
	pCtx = pCtx.WithPolicy(policy)

	resp := e.ApplyBackgroundChecks(context.Background(), pCtx)
	require.Len(t, resp.PolicyResponse.Rules, 2)

	assert.Equal(t, engineapi.RuleStatusSkip, resp.PolicyResponse.Rules[0].Status())
	assert.Equal(t, engineapi.RuleStatusPass, resp.PolicyResponse.Rules[1].Status(),
		"rule 2 must observe new vs old request.object; got %q", resp.PolicyResponse.Rules[1].Message())
}
