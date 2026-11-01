package engine

import (
	"context"
	"slices"
	"strings"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// fakeVpolClient serves a single ValidatingPolicy by name and a fixed list of
// PolicyExceptions, standing in for a controller-runtime manager cache that has
// already observed both objects (e.g. because it is the same cache that delivered
// the watch event which triggered reconciliation).
type fakeVpolClient struct {
	client.Client
	policy     *policiesv1beta1.ValidatingPolicy
	exceptions []policiesv1beta1.PolicyException
}

func (f *fakeVpolClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if f.policy != nil && key.Name == f.policy.Name {
		if o, ok := obj.(*policiesv1beta1.ValidatingPolicy); ok {
			*o = *f.policy
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

func (f *fakeVpolClient) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	if l, ok := list.(*policiesv1beta1.PolicyExceptionList); ok {
		l.Items = f.exceptions
		return nil
	}
	return nil
}

// fakeClient serves a fixed ValidatingPolicy by name so the reconciler can be driven without an API
// server.
type fakeClient struct {
	client.Client
	policy *policiesv1beta1.ValidatingPolicy
}

func (f *fakeClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if vp, ok := obj.(*policiesv1beta1.ValidatingPolicy); ok && f.policy != nil && key.Name == f.policy.Name {
		*vp = *f.policy
		return nil
	}
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

// staleExceptionLister always reports no exceptions. It stands in for the
// pre-fix wiring, where exceptions were read through a separate client-go
// SharedInformerFactory lister that is not guaranteed to have caught up with the
// controller-runtime manager cache that triggered the reconcile (issue #16989).
type staleExceptionLister struct{}

func (staleExceptionLister) List(labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, nil
}

// denyMissingTeamLabelPolicy is a JSON-mode ValidatingPolicy that fails any payload
// without a "team" field.
func denyMissingTeamLabelPolicy(name string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "has(object.team)", Message: "object must carry a team field"},
			},
		},
	}
}

// disallowLatestTagPolicy matches Pods and autogens onto both a built-in kind (deployments) and a
// custom workload CRD named by the explicit "<resource>.<version>.<group>" format, which does not
// need a live cluster to resolve.
func disallowLatestTagPolicy() *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disallow-latest-tag"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"deployments", "jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))"},
			},
		},
	}
}

// fullExemption is a PolicyException that unconditionally exempts every resource
// from policyName: no MatchConditions (always matches) and no Images/AllowedValues
// scoping (grants a full exemption rather than a partial CEL-visible allowlist).
func fullExemption(name, policyName string) policiesv1beta1.PolicyException {
	return policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policyName,
				Kind: policieskyvernoio.ValidatingPolicyKind,
			}},
		},
	}
}

// TestReconcile_ExceptionLister_StaleCacheMissesException reproduces the bug in
// issue #16989: when exceptions are looked up through a lister backed by a cache
// different from (and lagging behind) the one that triggered the reconcile, a
// PolicyException that already exists in the cluster is compiled out of the
// policy and the resource is denied as if the exception did not exist.
func TestReconcile_ExceptionLister_StaleCacheMissesException(t *testing.T) {
	ctx := context.Background()
	policy := denyMissingTeamLabelPolicy("require-team")
	fc := &fakeVpolClient{
		policy:     policy,
		exceptions: []policiesv1beta1.PolicyException{fullExemption("exempt-all", policy.Name)},
	}

	// The stale lister never sees the exception that fc (the reconciling cache)
	// already has, mirroring the dual-cache race.
	rec := newReconciler(compiler.NewCompiler(), fc, staleExceptionLister{}, true)
	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	eng := NewEngine(rec, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "no-team-field"}}
	resp, err := eng.Handle(ctx, celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	// Bug reproduced: the exception is silently ignored and the rule fails.
	require.Equal(t, engineapi.RuleStatusFail, resp.Policies[0].Rules[0].Status(),
		"a stale exception lister must reproduce the pre-fix behavior of ignoring an existing PolicyException")
}

// TestReconcile_ManagerCacheExceptionLister_SeesException verifies the fix: sourcing
// exceptions from the same manager-cache-backed reader used to fetch the policy
// (celengine.NewManagerPolicyExceptionLister) guarantees a PolicyException that
// exists by the time reconciliation runs is always picked up, because
// controller-runtime only invokes watch handlers after the cache's local store has
// already been updated.
func TestReconcile_ManagerCacheExceptionLister_SeesException(t *testing.T) {
	ctx := context.Background()
	policy := denyMissingTeamLabelPolicy("require-team")
	fc := &fakeVpolClient{
		policy:     policy,
		exceptions: []policiesv1beta1.PolicyException{fullExemption("exempt-all", policy.Name)},
	}

	polexLister := celengine.NewManagerPolicyExceptionLister(fc, "")
	rec := newReconciler(compiler.NewCompiler(), fc, polexLister, true)
	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	eng := NewEngine(rec, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "no-team-field"}}
	resp, err := eng.Handle(ctx, celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	// Fixed: the exception is honored and the rule is skipped instead of failing.
	require.Equal(t, engineapi.RuleStatusSkip, resp.Policies[0].Rules[0].Status(),
		"the manager-cache-backed lister must honor a PolicyException present in the same cache used to trigger reconciliation")
}

// TestReconcile_ExtractionMode is the regression test for the gap that let the JobSet extraction
// mechanism ship without actually working end to end: provider.go (used by NewProvider, the CLI/test
// path) set Policy.ExtractionMode correctly, but reconciler.go (used by the live cluster controller)
// had its own separate copy of the same loop that never set it, so the live admission path evaluated
// the custom-CRD target directly against the real object instead of extracting its pod template -
// exactly the "no such key: containers" failure this test would have caught.
func TestReconcile_ExtractionMode(t *testing.T) {
	ctx := context.Background()
	rec := newReconciler(
		compiler.NewCompiler(),
		&fakeClient{policy: disallowLatestTagPolicy()},
		nil, false,
	)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "disallow-latest-tag"}})
	require.NoError(t, err)

	fetched, err := rec.Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, fetched, 3, "expected the base policy plus one autogen'd variant per ReplacementsRef group")

	var sawDeploymentVariant, sawJobSetVariant bool
	for _, p := range fetched {
		mc := p.Policy.GetValidatingPolicySpec().MatchConstraints
		if mc == nil || len(mc.ResourceRules) == 0 {
			continue // the base, Pod-targeted policy
		}
		resources := mc.ResourceRules[0].Resources
		switch {
		case len(resources) > 0 && resources[0] == "deployments":
			sawDeploymentVariant = true
			assert.False(t, p.ExtractionMode, "the built-in deployments variant must not be extraction-mode")
		case len(resources) > 0 && resources[0] == "jobsets":
			sawJobSetVariant = true
			assert.True(t, p.ExtractionMode, "the custom-CRD jobsets variant must be extraction-mode")
		}
	}
	assert.True(t, sawDeploymentVariant, "expected a deployments autogen variant")
	assert.True(t, sawJobSetVariant, "expected a jobsets autogen variant")
}

// TestReconcile_AutogeneratedExceptionRewriting verifies that PolicyExceptions containing
// path-based CEL expressions (e.g., `request.object.spec`) are correctly rewritten when evaluated
// against autogenerated workloads (like Deployments) in the live Reconciler loop.
func TestReconcile_AutogeneratedExceptionRewriting(t *testing.T) {
	ctx := context.Background()
	policy := disallowLatestTagPolicy() // targets Pods, autogens Deployments
	// Ensure we can evaluate a raw Unstructured payload without Kubernetes admission attributes.
	policy.Spec.EvaluationConfiguration = &policiesv1beta1.EvaluationConfiguration{
		Mode: policieskyvernoio.EvaluationModeJSON,
	}

	// Create a PolicyException with a path-based MatchCondition (object.spec...).
	// This exception should exempt Deployments only if the rewrite successfully updates it to
	// `object.spec.template.spec...`.
	exception := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt-nginx"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policy.Name,
				Kind: policieskyvernoio.ValidatingPolicyKind,
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "match-nginx",
					Expression: "object.spec.containers.exists(c, c.name == 'nginx')",
				},
			},
		},
	}

	fc := &fakeVpolClient{
		policy:     policy,
		exceptions: []policiesv1beta1.PolicyException{exception},
	}

	polexLister := celengine.NewManagerPolicyExceptionLister(fc, "")
	rec := newReconciler(compiler.NewCompiler(), fc, polexLister, true)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	eng := NewEngine(rec, nil, nil)

	// Construct a payload for a Deployment whose container is named nginx.
	payload := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "nginx-deploy", "namespace": "default"},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "nginx", "image": "nginx:latest"},
						},
					},
				},
			},
		},
	}

	req := celengine.RequestFromJSON(nil, payload)
	resp, err := eng.Handle(ctx, req, nil)
	require.NoError(t, err)

	require.Len(t, resp.Policies, 3, "disallowLatestTagPolicy generates 3 variants (Pod, Deployment, JobSet)")

	// Find the deployment variant and assert it skipped.
	var checked bool
	for _, p := range resp.Policies {
		mc := p.Policy.GetValidatingPolicySpec().MatchConstraints
		if mc != nil && len(mc.ResourceRules) > 0 {
			resources := mc.ResourceRules[0].Resources
			if len(resources) > 0 && resources[0] == "deployments" {
				for _, r := range p.Rules {
					if r.Status() == engineapi.RuleStatusSkip && strings.Contains(r.Message(), "exempt-nginx") {
						checked = true
					}
				}
			}
		}
	}
	require.True(t, checked, "a path-based exception condition must be rewritten to match the structure of autogenerated variants via the Reconciler (expected an exception skip)")

	// Construct a payload for a Deployment whose container is NOT named nginx (negative case).
	payloadNegative := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "other-deploy", "namespace": "default"},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "other", "image": "nginx:latest"},
						},
					},
				},
			},
		},
	}

	reqNegative := celengine.RequestFromJSON(nil, payloadNegative)
	respNegative, err := eng.Handle(ctx, reqNegative, nil)
	require.NoError(t, err)

	var checkedNegative bool
	for _, p := range respNegative.Policies {
		mc := p.Policy.GetValidatingPolicySpec().MatchConstraints
		if mc != nil && len(mc.ResourceRules) > 0 {
			resources := mc.ResourceRules[0].Resources
			if len(resources) > 0 && resources[0] == "deployments" {
				for _, r := range p.Rules {
					if r.Status() == engineapi.RuleStatusFail {
						checkedNegative = true
					}
					require.NotEqual(t, engineapi.RuleStatusSkip, r.Status(), "expected the negative case not to skip")
				}
			}
		}
	}
	require.True(t, checkedNegative, "a deployment without an nginx container should fail the policy and not be exempted")
}

// A policy that no longer compiles must be dropped from the cache instead of leaving the previous
// compiled version silently enforced.
func TestReconcile_CompileFailureRemovesPolicyFromCache(t *testing.T) {
	ctx := context.Background()
	vp := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "bad",
				Expression: "1 + 'a'", // Invalid CEL expression
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.spec.containers.size() > 0",
			}},
		},
	}
	rec := newReconciler(compiler.NewCompiler(), &fakeClient{policy: vp}, nil, false)
	name := types.NamespacedName{Name: vp.Name}
	// Pre-populate the cache as if an earlier version had compiled.
	rec.policies[name.String()] = []Policy{{}}

	res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
	require.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, res)

	_, exists := rec.policies[name.String()]
	assert.False(t, exists, "a policy that fails to compile must be removed from the cache")
}

// failJobSetVariant compiles policies with the real compiler but fails the autogen variant that
// targets jobsets, standing in for a variant whose rewritten expressions no longer compile.
type failJobSetVariant struct {
	compiler.Compiler
}

func (c failJobSetVariant) Compile(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*compiler.Policy, field.ErrorList) {
	if mc := policy.GetValidatingPolicySpec().MatchConstraints; mc != nil && len(mc.ResourceRules) > 0 && slices.Contains(mc.ResourceRules[0].Resources, "jobsets") {
		return nil, field.ErrorList{field.Invalid(field.NewPath("spec"), "jobsets", "variant does not compile")}
	}
	return c.Compiler.Compile(policy, exceptions)
}

// An autogen variant that fails to compile is skipped instead of discarding the whole reconcile, so
// the base policy and the variants that do compile stay cached and enforced.
func TestReconcile_AutogenVariantCompileFailure(t *testing.T) {
	ctx := context.Background()
	policy := disallowLatestTagPolicy()
	rec := newReconciler(failJobSetVariant{compiler.NewCompiler()}, &fakeClient{policy: policy}, nil, false)

	res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, res)

	fetched, err := rec.Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, fetched, 2, "the base policy and the deployments variant must stay cached")
	for _, p := range fetched {
		assert.NotContains(t, p.Policy.GetValidatingPolicySpec().MatchConstraints.ResourceRules[0].Resources, "jobsets",
			"the variant that failed to compile must not be cached")
	}
}
