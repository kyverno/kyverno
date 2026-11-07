package libs

import (
	"context"
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
)

// staticEntry is a store.Entry that always returns the same value.
type staticEntry struct {
	data any
}

func (e *staticEntry) Get(projection string) (any, error) {
	return e.data, nil
}

func (e *staticEntry) Stop() {}

// TestContextProvider_GetGlobalReference_KubernetesShapedEntry covers globalContext.get on an
// entry holding a Kubernetes list, which failed to convert to a CEL value after cel-go v0.31.
func TestContextProvider_GetGlobalReference_KubernetesShapedEntry(t *testing.T) {
	gctxStore := store.New(0)
	// Same shape as the deleting-policies/cel-lib/globalcontext-lib conformance fixture.
	require.NoError(t, gctxStore.Set("gctxentry-apicall-correct", &staticEntry{
		data: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "DeploymentList",
			"items": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "test-globalcontext-apicall-correct",
					},
				},
			},
		},
	}))

	cp := &contextProvider{gctxStore: gctxStore}

	// Same env the policy compilers build for the globalcontext library.
	env, err := cel.NewEnv(
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: cp},
			version.MajorMinor(1, 18),
		),
	)
	require.NoError(t, err)

	ast, iss := env.Compile(`globalContext.get("gctxentry-apicall-correct", "") != 0`)
	require.NoError(t, iss.Err(), "policy expression must compile")

	prg, err := env.Program(ast)
	require.NoError(t, err)

	_, _, err = prg.Eval(map[string]any{
		"globalContext": globalcontext.Context{ContextInterface: cp},
	})
	require.NoError(t, err, "globalContext.get on a Kubernetes-shaped entry must not fail at evaluation time")
}

// failingEntry is a store.Entry whose Get always fails.
type failingEntry struct{}

func (e *failingEntry) Get(projection string) (any, error) {
	return nil, errors.New("api call failed")
}

func (e *failingEntry) Stop() {}

func TestContextProvider_GetGlobalReference(t *testing.T) {
	gctxStore := store.New(0)
	require.NoError(t, gctxStore.Set("failing", &failingEntry{}))
	require.NoError(t, gctxStore.Set("scalar", &staticEntry{data: "value"}))
	require.NoError(t, gctxStore.Set("kube-object", &staticEntry{data: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm"},
	}}))
	cp := &contextProvider{gctxStore: gctxStore}

	tests := []struct {
		name    string
		entry   string
		want    any
		wantErr string
	}{{
		name:  "an absent entry gives no value and no error",
		entry: "missing",
		want:  nil,
	}, {
		name:    "an entry that fails returns its error",
		entry:   "failing",
		wantErr: "api call failed",
	}, {
		name:  "a value that is not a Kubernetes object is returned as is",
		entry: "scalar",
		want:  "value",
	}, {
		name:  "a Kubernetes object is returned as a map",
		entry: "kube-object",
		want: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "cm"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cp.GetGlobalReference(tt.entry, "")
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// An error from the entry must fail the CEL evaluation instead of yielding a value.
func TestContextProvider_GetGlobalReference_EntryErrorFailsEvaluation(t *testing.T) {
	gctxStore := store.New(0)
	require.NoError(t, gctxStore.Set("failing", &failingEntry{}))
	cp := &contextProvider{gctxStore: gctxStore}
	env, err := cel.NewEnv(
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: cp},
			version.MajorMinor(1, 18),
		),
	)
	require.NoError(t, err)
	ast, iss := env.Compile(`globalContext.get("failing", "") != 0`)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)
	_, _, err = prg.Eval(map[string]any{
		"globalContext": globalcontext.Context{ContextInterface: cp},
	})
	require.ErrorContains(t, err, "api call failed")
}

// TestAddGenerateLabels_NeverWritesLegacyOnlyLabels pins that the CEL
// generate path never writes GenerateRuleLabel or GeneratePolicyNamespaceLabel
// onto downstream resources. The documented cleanup command for orphaned
// legacy-generated resources (see #17707 release notes) selects on
// `generate.kyverno.io/rule-name` specifically because gpol-managed
// resources never carry it; if this test starts failing, that selector
// would start deleting CEL-managed resources too.
func TestAddGenerateLabels_NeverWritesLegacyOnlyLabels(t *testing.T) {
	cp := &contextProvider{}
	cp.SetGenerateContext(
		"my-gpol", "", "trigger-name", "trigger-ns", "v1", "apps", "Deployment", "trigger-uid",
		false, false,
	)
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	cp.addGenerateLabels(obj)

	labels := obj.GetLabels()
	_, hasRuleLabel := labels[common.GenerateRuleLabel]
	_, hasPolicyNamespaceLabel := labels[common.GeneratePolicyNamespaceLabel]
	assert.False(t, hasRuleLabel, "addGenerateLabels must never write %s; the orphan-cleanup selector relies on its absence", common.GenerateRuleLabel)
	assert.False(t, hasPolicyNamespaceLabel, "addGenerateLabels must never write %s", common.GeneratePolicyNamespaceLabel)
	// Sanity: the labels it does write are still present.
	assert.Equal(t, "my-gpol", labels[common.GeneratePolicyLabel])
}

// TestCELGeneratePolicyScopeDisallowsCrossScopeAdoption ensures a cluster
// GeneratingPolicy and a NamespacedGeneratingPolicy of the same name and
// trigger UID cannot adopt each other's newly generated resources.
func TestCELGeneratePolicyScopeDisallowsCrossScopeAdoption(t *testing.T) {
	for _, tc := range []struct {
		name      string
		namespace string
		wantScope string
	}{
		{name: "cluster", wantScope: common.GeneratePolicyScopeCluster},
		{name: "namespaced", namespace: "team-a", wantScope: common.GeneratePolicyScopeNamespaced},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := &contextProvider{}
			cp.SetGenerateContext("same-name", tc.namespace, "trigger", "team-a", "v1", "", "ConfigMap", "same-trigger-uid", false, false)
			obj := &unstructured.Unstructured{Object: map[string]any{}}
			cp.addGenerateLabels(obj)
			require.Equal(t, tc.wantScope, obj.GetLabels()[common.GeneratePolicyScopeLabel])
			require.True(t, cp.isManagedByPolicy(context.Background(), obj))

			other := &contextProvider{}
			otherNamespace := ""
			if tc.namespace == "" {
				otherNamespace = "team-a"
			}
			other.SetGenerateContext("same-name", otherNamespace, "trigger", "team-a", "v1", "", "ConfigMap", "same-trigger-uid", false, false)
			require.False(t, other.isManagedByPolicy(context.Background(), obj))
		})
	}
}

// conflictingPolicyLookup answers whether the other scope owns a same-name
// policy. Legacy resources can only be migrated when that ambiguity is absent.
type conflictingPolicyLookup struct {
	dclient.Interface
	conflict bool
}

func (c conflictingPolicyLookup) GetResource(_ context.Context, _, _, _, _ string, _ ...string) (*unstructured.Unstructured, error) {
	if c.conflict {
		return &unstructured.Unstructured{Object: map[string]any{"kind": "GeneratingPolicy"}}, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "policies.kyverno.io", Resource: "generatingpolicies"}, "same-name")
}

func TestLegacyCELDownstreamAdoptionChecksConflictingPolicyScope(t *testing.T) {
	for _, policyNamespace := range []string{"", "team-a"} {
		t.Run("policy-namespace="+policyNamespace, func(t *testing.T) {
			labels := map[string]string{
				kyverno.LabelAppManagedBy:       kyverno.ValueKyvernoApp,
				common.GeneratePolicyLabel:      "same-name",
				common.GenerateTriggerNSLabel:   "team-a",
				common.GenerateTriggerUIDLabel:  "same-trigger-uid",
			}
			obj := &unstructured.Unstructured{Object: map[string]any{}}
			obj.SetLabels(labels)
			client := &conflictingPolicyLookup{}
			cp := &contextProvider{client: client}
			cp.SetGenerateContext("same-name", policyNamespace, "trigger", "team-a", "v1", "", "ConfigMap", "same-trigger-uid", false, false)
			require.True(t, cp.isManagedByPolicy(context.Background(), obj), "historical resource may be adopted when no counterpart exists")
			client.conflict = true
			require.False(t, cp.isManagedByPolicy(context.Background(), obj), "same-name policy in other scope prevents ambiguous adoption")
			client.conflict = false
			if policyNamespace == "" {
				labels[common.GeneratePolicyScopeLabel] = common.GeneratePolicyScopeCluster
			} else {
				labels[common.GeneratePolicyScopeLabel] = common.GeneratePolicyScopeNamespaced
			}
			obj.SetLabels(labels)
			require.True(t, cp.isManagedByPolicy(context.Background(), obj), "explicitly scoped object does not need a competing-policy lookup")
		})
	}
}

// A namespace (or other cluster-scoped trigger) cannot be owned by a
// NamespacedGeneratingPolicy, so old CEL downstreams need no counterpart GET.
func TestLegacyCELClusterTriggerRemainsAdoptable(t *testing.T) {
	cp := &contextProvider{}
	cp.SetGenerateContext("same-name", "", "trigger", "", "v1", "", "Namespace", "trigger-uid", false, false)
	obj := &unstructured.Unstructured{Object: map[string]any{}}
	obj.SetLabels(map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     "same-name",
		common.GenerateTriggerUIDLabel: "trigger-uid",
	})
	require.True(t, cp.isManagedByPolicy(context.Background(), obj))
}
