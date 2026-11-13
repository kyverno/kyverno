package webhook

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestGetValidatingPoliciesSkipsForeignMode(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range validatingPolicies("") {
		require.NoError(t, indexer.Add(pol))
	}

	c := &controller{vpolLister: policiesv1beta1listers.NewValidatingPolicyLister(indexer)}
	got, err := c.getValidatingPolicies()
	require.NoError(t, err)
	assertPolicyNames(t, got, "owned", "kubernetes", "json")
}

func validatingPolicies(namespace string) []*policiesv1beta1.ValidatingPolicy {
	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: namespace}
	}
	return []*policiesv1beta1.ValidatingPolicy{
		{ObjectMeta: meta("owned")},
		{
			ObjectMeta: meta("kubernetes"),
			Spec: policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Kubernetes"},
			},
		},
		{
			ObjectMeta: meta("json"),
			Spec: policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "JSON"},
			},
		},
		{
			ObjectMeta: meta("envoy"),
			Spec: policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Envoy"},
			},
		},
		{
			ObjectMeta: meta("extproc"),
			Spec: policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "ext_proc"},
			},
		},
	}
}

func assertPolicyNames(t *testing.T, policies []engineapi.GenericPolicy, names ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, policy := range policies {
		got[policy.GetName()] = true
	}
	want := map[string]bool{}
	for _, name := range names {
		want[name] = true
	}
	assert.Equal(t, want, got)
}

func TestGetPoliciesSkipForeignMode(t *testing.T) {
	meta := func(name, namespace string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: namespace}
	}
	eval := func(mode policiesv1beta1.EvaluationMode) *policiesv1beta1.EvaluationConfiguration {
		return &policiesv1beta1.EvaluationConfiguration{Mode: mode}
	}
	meval := func(mode policiesv1beta1.EvaluationMode) *policiesv1beta1.MutatingPolicyEvaluationConfiguration {
		return &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: mode}
	}

	nvpol := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range []*policiesv1beta1.NamespacedValidatingPolicy{
		{ObjectMeta: meta("owned", "ns")},
		{ObjectMeta: meta("kubernetes", "ns"), Spec: policiesv1beta1.ValidatingPolicySpec{EvaluationConfiguration: eval("Kubernetes")}},
		{ObjectMeta: meta("json", "ns"), Spec: policiesv1beta1.ValidatingPolicySpec{EvaluationConfiguration: eval("JSON")}},
		{ObjectMeta: meta("envoy", "ns"), Spec: policiesv1beta1.ValidatingPolicySpec{EvaluationConfiguration: eval("Envoy")}},
		{ObjectMeta: meta("extproc", "ns"), Spec: policiesv1beta1.ValidatingPolicySpec{EvaluationConfiguration: eval("ext_proc")}},
	} {
		require.NoError(t, nvpol.Add(pol))
	}
	ivpol := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range []*policiesv1beta1.ImageValidatingPolicy{
		{ObjectMeta: meta("owned", "")},
		{ObjectMeta: meta("kubernetes", ""), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("Kubernetes")}},
		{ObjectMeta: meta("json", ""), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("JSON")}},
		{ObjectMeta: meta("envoy", ""), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("Envoy")}},
		{ObjectMeta: meta("extproc", ""), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("ext_proc")}},
	} {
		require.NoError(t, ivpol.Add(pol))
	}
	nivpol := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range []*policiesv1beta1.NamespacedImageValidatingPolicy{
		{ObjectMeta: meta("owned", "ns")},
		{ObjectMeta: meta("kubernetes", "ns"), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("Kubernetes")}},
		{ObjectMeta: meta("json", "ns"), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("JSON")}},
		{ObjectMeta: meta("envoy", "ns"), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("Envoy")}},
		{ObjectMeta: meta("extproc", "ns"), Spec: policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: eval("ext_proc")}},
	} {
		require.NoError(t, nivpol.Add(pol))
	}
	mpol := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range []*policiesv1beta1.MutatingPolicy{
		{ObjectMeta: meta("owned", "")},
		{ObjectMeta: meta("kubernetes", ""), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("Kubernetes")}},
		{ObjectMeta: meta("json", ""), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("JSON")}},
		{ObjectMeta: meta("envoy", ""), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("Envoy")}},
		{ObjectMeta: meta("extproc", ""), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("ext_proc")}},
	} {
		require.NoError(t, mpol.Add(pol))
	}
	nmpol := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range []*policiesv1beta1.NamespacedMutatingPolicy{
		{ObjectMeta: meta("owned", "ns")},
		{ObjectMeta: meta("kubernetes", "ns"), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("Kubernetes")}},
		{ObjectMeta: meta("json", "ns"), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("JSON")}},
		{ObjectMeta: meta("envoy", "ns"), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("Envoy")}},
		{ObjectMeta: meta("extproc", "ns"), Spec: policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: meval("ext_proc")}},
	} {
		require.NoError(t, nmpol.Add(pol))
	}

	c := &controller{
		nvpolLister:  policiesv1beta1listers.NewNamespacedValidatingPolicyLister(nvpol),
		ivpolLister:  policiesv1beta1listers.NewImageValidatingPolicyLister(ivpol),
		nivpolLister: policiesv1beta1listers.NewNamespacedImageValidatingPolicyLister(nivpol),
		mpolLister:   policiesv1beta1listers.NewMutatingPolicyLister(mpol),
		nmpolLister:  policiesv1beta1listers.NewNamespacedMutatingPolicyLister(nmpol),
	}
	assertOwned := func(t *testing.T, policies []engineapi.GenericPolicy, err error) {
		t.Helper()
		require.NoError(t, err)
		assertPolicyNames(t, policies, "owned", "kubernetes", "json")
	}
	got, err := c.getNamespacedValidatingPolicies()
	assertOwned(t, got, err)
	got, err = c.getImageValidatingPolicies()
	assertOwned(t, got, err)
	got, err = c.getNamespacedImageValidatingPolicies()
	assertOwned(t, got, err)
	got, err = c.getMutatingPolicies()
	assertOwned(t, got, err)
	got, err = c.getNamespacedMutatingPolicies()
	assertOwned(t, got, err)
}
