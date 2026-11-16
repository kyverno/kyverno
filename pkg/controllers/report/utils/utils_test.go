package utils

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestReportsAreIdentical_EmptyReports(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	after := &reportsv1.EphemeralReport{}

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "empty reports should be identical")
}

func TestReportsAreIdentical_SameResults(t *testing.T) {
	results := []openreportsv1alpha1.ReportResult{
		{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreportsv1alpha1.Result(openreports.StatusPass),
		},
	}
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "report1",
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"key": "value"},
		},
	}
	before.SetResults(results)

	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "report1",
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"key": "value"},
		},
	}
	after.SetResults(results)

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "reports with same results should be identical")
}

func TestReportsAreIdentical_DifferentResults(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusFail)},
	})

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different results should not be identical")
}

func TestReportsAreIdentical_DifferentResultCount(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
		{Policy: "policy2", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different result count should not be identical")
}

func TestReportsAreIdentical_DifferentLabels(t *testing.T) {
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "test"},
		},
	}
	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "different"},
		},
	}

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different labels should not be identical")
}

func TestReportsAreIdentical_DifferentAnnotations(t *testing.T) {
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "value1"},
		},
	}
	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "value2"},
		},
	}

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different annotations should not be identical")
}

func TestReportsAreIdentical_TimestampIgnored(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{
			Policy:    "policy1",
			Result:    openreportsv1alpha1.Result(openreports.StatusPass),
			Timestamp: metav1.Timestamp{Seconds: 1000},
		},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{
			Policy:    "policy1",
			Result:    openreportsv1alpha1.Result(openreports.StatusPass),
			Timestamp: metav1.Timestamp{Seconds: 2000},
		},
	})

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "reports with same content but different timestamps should be identical")
}

func TestGetExcludeReportingLabelRequirement(t *testing.T) {
	req, err := getExcludeReportingLabelRequirement()
	assert.NoError(t, err)
	assert.NotNil(t, req)
}

func TestGetIncludeReportingLabelRequirement(t *testing.T) {
	req, err := getIncludeReportingLabelRequirement()
	assert.NoError(t, err)
	assert.NotNil(t, req)
}

func TestFetchValidatingPolicies_SkipsPoliciesOwnedByOtherControllers(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for name, mode := range map[string]string{"kyverno": "", "json": "JSON", "envoy": "Envoy", "http": "HTTP"} {
		assert.NoError(t, indexer.Add(&policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: policiesv1beta1.ValidatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: mode},
			},
		}))
	}
	policies, err := FetchValidatingPolicies(policiesv1beta1listers.NewValidatingPolicyLister(indexer))
	assert.NoError(t, err)
	names := make([]string, 0, len(policies))
	for _, policy := range policies {
		names = append(names, policy.Name)
	}
	assert.ElementsMatch(t, []string{"kyverno", "json"}, names)
}

// policyNames returns namespace/name for each policy, so tests can check which namespace it came from.
func policyNames[T any, P interface {
	*T
	metav1.Object
}](policies []T,
) []string {
	names := make([]string, 0, len(policies))
	for i := range policies {
		var p P = &policies[i]
		names = append(names, p.GetNamespace()+"/"+p.GetName())
	}
	return names
}

// TestFetchPolicies_SkipsPoliciesOwnedByOtherControllers covers the other report fetch
// functions that filter by evaluation mode. Each indexer holds a Kyverno-owned and a
// foreign policy in two namespaces (or once, for cluster-scoped kinds).
func TestFetchPolicies_SkipsPoliciesOwnedByOtherControllers(t *testing.T) {
	newIndexer := func() cache.Indexer {
		return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	}
	vpolSpec := func(mode policiesv1beta1.EvaluationMode) policiesv1beta1.ValidatingPolicySpec {
		return policiesv1beta1.ValidatingPolicySpec{EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: mode}}
	}
	ivpolSpec := func(mode policiesv1beta1.EvaluationMode) policiesv1beta1.ImageValidatingPolicySpec {
		return policiesv1beta1.ImageValidatingPolicySpec{EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: mode}}
	}
	mpolSpec := func(mode policiesv1beta1.EvaluationMode) policiesv1beta1.MutatingPolicySpec {
		return policiesv1beta1.MutatingPolicySpec{EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: mode}}
	}
	namespaces := []string{"ns1", "ns2"}
	modes := map[string]policiesv1beta1.EvaluationMode{"kyverno": "Kubernetes", "envoy": "Envoy"}

	t.Run("ImageValidatingPolicy", func(t *testing.T) {
		indexer := newIndexer()
		for name, mode := range modes {
			assert.NoError(t, indexer.Add(&policiesv1beta1.ImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: ivpolSpec(mode)}))
		}
		policies, err := FetchImageVerificationPolicies(policiesv1beta1listers.NewImageValidatingPolicyLister(indexer))
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"/kyverno"}, policyNames(policies))
	})

	t.Run("MutatingPolicy", func(t *testing.T) {
		indexer := newIndexer()
		for name, mode := range modes {
			assert.NoError(t, indexer.Add(&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: mpolSpec(mode)}))
		}
		policies, err := FetchMutatingPolicies(policiesv1beta1listers.NewMutatingPolicyLister(indexer))
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"/kyverno"}, policyNames(policies))
	})

	namespacedTests := []struct {
		namespace string
		want      []string
	}{
		{namespace: "ns1", want: []string{"ns1/kyverno"}},
		{namespace: "", want: []string{"ns1/kyverno", "ns2/kyverno"}},
	}

	t.Run("NamespacedValidatingPolicy", func(t *testing.T) {
		indexer := newIndexer()
		for _, ns := range namespaces {
			for name, mode := range modes {
				assert.NoError(t, indexer.Add(&policiesv1beta1.NamespacedValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, Spec: vpolSpec(mode)}))
			}
		}
		for _, tt := range namespacedTests {
			policies, err := FetchNamespacedValidatingPolicies(policiesv1beta1listers.NewNamespacedValidatingPolicyLister(indexer), tt.namespace)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, policyNames(policies), "namespace %q", tt.namespace)
		}
	})

	t.Run("NamespacedImageValidatingPolicy", func(t *testing.T) {
		indexer := newIndexer()
		for _, ns := range namespaces {
			for name, mode := range modes {
				assert.NoError(t, indexer.Add(&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, Spec: ivpolSpec(mode)}))
			}
		}
		for _, tt := range namespacedTests {
			policies, err := FetchNamespacedImageVerificationPolicies(policiesv1beta1listers.NewNamespacedImageValidatingPolicyLister(indexer), tt.namespace)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, policyNames(policies), "namespace %q", tt.namespace)
		}
	})

	t.Run("NamespacedMutatingPolicy", func(t *testing.T) {
		indexer := newIndexer()
		for _, ns := range namespaces {
			for name, mode := range modes {
				assert.NoError(t, indexer.Add(&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, Spec: mpolSpec(mode)}))
			}
		}
		for _, tt := range namespacedTests {
			policies, err := FetchNamespacedMutatingPolicies(policiesv1beta1listers.NewNamespacedMutatingPolicyLister(indexer), tt.namespace)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, policyNames(policies), "namespace %q", tt.namespace)
		}
	})
}
