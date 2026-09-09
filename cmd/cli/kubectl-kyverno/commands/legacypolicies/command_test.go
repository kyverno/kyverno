package legacypolicies

import (
	"bytes"
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

// crdSpecs mirrors the group, stored version, plural, and scope of each
// legacy kind as they are actually shipped, so the fixtures built from it
// exercise the same "read scope/version off the CRD" path as production.
var crdSpecs = map[string]struct {
	Kind    string
	Plural  string
	Version string
	Scope   apiextensionsv1.ResourceScope
}{
	"clusterpolicies.kyverno.io":        {Kind: "ClusterPolicy", Plural: "clusterpolicies", Version: "v1", Scope: apiextensionsv1.ClusterScoped},
	"policies.kyverno.io":               {Kind: "Policy", Plural: "policies", Version: "v1", Scope: apiextensionsv1.NamespaceScoped},
	"cleanuppolicies.kyverno.io":        {Kind: "CleanupPolicy", Plural: "cleanuppolicies", Version: "v2", Scope: apiextensionsv1.NamespaceScoped},
	"clustercleanuppolicies.kyverno.io": {Kind: "ClusterCleanupPolicy", Plural: "clustercleanuppolicies", Version: "v2", Scope: apiextensionsv1.ClusterScoped},
	"policyexceptions.kyverno.io":       {Kind: "PolicyException", Plural: "policyexceptions", Version: "v2", Scope: apiextensionsv1.NamespaceScoped},
}

// gvrToListKind mirrors every legacy kind's GVR so the fake dynamic client
// knows how to build a List response for each of them.
var gvrToListKind = map[schema.GroupVersionResource]string{
	{Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"}:        "ClusterPolicyList",
	{Group: "kyverno.io", Version: "v1", Resource: "policies"}:               "PolicyList",
	{Group: "kyverno.io", Version: "v2", Resource: "cleanuppolicies"}:        "CleanupPolicyList",
	{Group: "kyverno.io", Version: "v2", Resource: "clustercleanuppolicies"}: "ClusterCleanupPolicyList",
	{Group: "kyverno.io", Version: "v2", Resource: "policyexceptions"}:       "PolicyExceptionList",
}

func crdFor(lk legacyKind) *apiextensionsv1.CustomResourceDefinition {
	spec := crdSpecs[lk.CRDName]
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: lk.CRDName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kyverno.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: spec.Plural,
				Kind:   spec.Kind,
			},
			Scope: spec.Scope,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{Name: spec.Version, Storage: true, Served: true},
			},
		},
	}
}

func unstructuredObj(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}

func TestCheckLegacyPolicies_NoCRDsRegistered(t *testing.T) {
	apiClient := apiextensionsfake.NewSimpleClientset()
	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind)

	results, total, err := checkLegacyPolicies(context.Background(), apiClient, dynClient)

	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Len(t, results, len(legacyKinds))
	for _, result := range results {
		require.Equal(t, 0, result.Count)
	}
}

func TestCheckLegacyPolicies_CRDsRegisteredNoInstances(t *testing.T) {
	var crds []runtime.Object
	for _, lk := range legacyKinds {
		crds = append(crds, crdFor(lk))
	}
	apiClient := apiextensionsfake.NewSimpleClientset(crds...)
	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind)

	results, total, err := checkLegacyPolicies(context.Background(), apiClient, dynClient)

	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Len(t, results, len(legacyKinds))
}

func TestCheckLegacyPolicies_FindsInstances(t *testing.T) {
	var crds []runtime.Object
	for _, lk := range legacyKinds {
		crds = append(crds, crdFor(lk))
	}
	apiClient := apiextensionsfake.NewSimpleClientset(crds...)

	objs := []runtime.Object{
		unstructuredObj("kyverno.io/v1", "ClusterPolicy", "", "cpol-1"),
		unstructuredObj("kyverno.io/v1", "ClusterPolicy", "", "cpol-2"),
		unstructuredObj("kyverno.io/v1", "Policy", "team-a", "pol-1"),
		unstructuredObj("kyverno.io/v2", "PolicyException", "team-b", "polex-1"),
	}
	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objs...)

	results, total, err := checkLegacyPolicies(context.Background(), apiClient, dynClient)

	require.NoError(t, err)
	require.Equal(t, 4, total)

	byKind := map[string]kindResult{}
	for _, result := range results {
		byKind[result.Kind] = result
	}
	require.Equal(t, 2, byKind["ClusterPolicy"].Count)
	require.Equal(t, 1, byKind["Policy"].Count)
	require.Contains(t, byKind["Policy"].Names, "team-a/pol-1")
	require.Equal(t, 1, byKind["PolicyException"].Count)
	require.Contains(t, byKind["PolicyException"].Names, "team-b/polex-1")
	require.Equal(t, 0, byKind["CleanupPolicy"].Count)
	require.Equal(t, 0, byKind["ClusterCleanupPolicy"].Count)
}

// TestCheckLegacyPolicies_CRDRegisteredListFails verifies that once a CRD
// is confirmed to exist, a failure listing its instances is surfaced as an
// error rather than silently counted as zero - a false "zero" here would
// let legacy resources slip past the gate undetected. This covers, in
// particular, a List call against the CRD's stored version returning
// NotFound (for example because the CRD does not actually serve that
// version): that must not be swallowed the way an absent CRD is.
func TestCheckLegacyPolicies_CRDRegisteredListFails(t *testing.T) {
	apiClient := apiextensionsfake.NewSimpleClientset(crdFor(legacyKinds[0]))
	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind)
	dynClient.PrependReactor("list", "clusterpolicies", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "")
	})

	_, _, err := checkLegacyPolicies(context.Background(), apiClient, dynClient)

	require.Error(t, err)
}

func TestRun_FailsWhenLegacyResourcesExist(t *testing.T) {
	apiClient := apiextensionsfake.NewSimpleClientset(crdFor(legacyKinds[0]))
	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind,
		unstructuredObj("kyverno.io/v1", "ClusterPolicy", "", "cpol-1"),
	)

	var out bytes.Buffer
	err := run(context.Background(), &out, apiClient, dynClient)

	require.Error(t, err)
	require.Contains(t, err.Error(), "found 1 legacy policy resource")
	require.Contains(t, out.String(), "ClusterPolicy: 1")
	require.Contains(t, out.String(), "cpol-1")
}

func TestRun_SucceedsWhenNoLegacyResources(t *testing.T) {
	apiClient := apiextensionsfake.NewSimpleClientset()
	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind)

	var out bytes.Buffer
	err := run(context.Background(), &out, apiClient, dynClient)

	require.NoError(t, err)
	require.Contains(t, out.String(), "no legacy policy resources found")
}

// TestCommandIsHidden locks in the maintainer's requirement that this
// command not appear in --help output or generated docs.
func TestCommandIsHidden(t *testing.T) {
	if !Command().Hidden {
		t.Error("check-legacy-policies must stay hidden")
	}
}

// TestLegacyKindsAreInDeprecationsTable is a drift guard tying the local
// legacyKinds list to the shared pkg/deprecations table, so the two do not
// silently diverge.
func TestLegacyKindsAreInDeprecationsTable(t *testing.T) {
	for _, lk := range legacyKinds {
		if _, ok := deprecations.BuildKindWarning("kyverno.io", "", lk.Kind); !ok {
			t.Errorf("kind %s is not in the deprecations table", lk.Kind)
		}
	}
}
