package test

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/filter"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterTestResults(t *testing.T) {
	pass := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "require-catalog-team",
			Kind:   "ConfigMap",
			Result: openreportsv1alpha1.Result("pass"),
		},
		TestResultData: v1alpha1.TestResultData{Resources: []string{"demo-blue", "demo-green"}},
	}
	fail := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "require-catalog-team",
			Kind:   "ConfigMap",
			Result: openreportsv1alpha1.Result("fail"),
		},
		TestResultData: v1alpha1.TestResultData{Resources: []string{"draft-gray"}},
	}
	// a result that declares no resource applies to every resource
	all := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "require-catalog-team",
			Kind:   "ConfigMap",
			Result: openreportsv1alpha1.Result("pass"),
		},
	}

	// the same result narrowed to the resource selected by resource=demo-blue
	onlyBlue := pass
	onlyBlue.Resources = []string{"demo-blue"}

	tests := []struct {
		name     string
		selector string
		want     []v1alpha1.TestResult
	}{{
		name:     "default selector keeps every result",
		selector: "policy=*,rule=*,resource=*",
		want:     []v1alpha1.TestResult{pass, fail, all},
	}, {
		name:     "resource selector narrows the declared resources",
		selector: "policy=*,rule=*,resource=demo-*",
		want:     []v1alpha1.TestResult{pass, all},
	}, {
		name:     "resource selector drops the results it does not select",
		selector: "policy=*,rule=*,resource=draft-*",
		want:     []v1alpha1.TestResult{fail, all},
	}, {
		// the sub filters of a selector are combined with AND, so a result has to
		// match every pattern; only the results that declare no resource survive
		name:     "several resource patterns must all match a result",
		selector: "policy=*,rule=*,resource=demo-green,resource=draft-*",
		want:     []v1alpha1.TestResult{all},
	}, {
		name:     "a selector that matches no resource keeps only the results that declare none",
		selector: "policy=*,rule=*,resource=missing-*",
		want:     []v1alpha1.TestResult{all},
	}, {
		name:     "resource selector narrows a result to a subset of its resources",
		selector: "policy=*,rule=*,resource=demo-blue",
		want:     []v1alpha1.TestResult{onlyBlue, all},
	}, {
		// a wildcard pattern selects every resource, it must not discard the
		// patterns that follow it
		name:     "a wildcard resource pattern keeps the patterns that follow it",
		selector: "policy=*,rule=*,resource=*,resource=demo-blue",
		want:     []v1alpha1.TestResult{onlyBlue, all},
	}, {
		name:     "a policy selector that matches nothing keeps nothing",
		selector: "policy=other-*,rule=*,resource=*",
		want:     nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, errs := filter.ParseFilter(tt.selector)
			require.Empty(t, errs)
			assert.Equal(t, tt.want, filterTestResults([]v1alpha1.TestResult{pass, fail, all}, selector, filter.ExtractResourceFilters(tt.selector)))
		})
	}
}

func TestNarrowResources(t *testing.T) {
	assert.Equal(t,
		[]string{"demo-blue", "demo-green"},
		narrowResources([]string{"demo-blue", "draft-gray", "demo-green"}, []string{"demo-*"}))
	assert.Nil(t, narrowResources([]string{"draft-gray"}, []string{"demo-*"}))
	assert.Equal(t,
		[]string{"a", "b"},
		narrowResources([]string{"a", "b"}, []string{"a", "b"}))
}

func TestNarrowResourceSpecs(t *testing.T) {
	spec := func(namespace, name string) v1alpha1.TestResourceSpec {
		return v1alpha1.TestResourceSpec{Version: "v1", Kind: "ConfigMap", Namespace: namespace, Name: name}
	}
	assert.Equal(t,
		[]v1alpha1.TestResourceSpec{spec("default", "demo-blue")},
		narrowResourceSpecs([]v1alpha1.TestResourceSpec{spec("default", "demo-blue"), spec("default", "draft-gray")}, []string{"demo-*"}))
	assert.Equal(t,
		[]v1alpha1.TestResourceSpec{spec("demo", "blue")},
		narrowResourceSpecs([]v1alpha1.TestResourceSpec{spec("demo", "blue"), spec("default", "blue")}, []string{"demo/blue"}))
	assert.Nil(t, narrowResourceSpecs([]v1alpha1.TestResourceSpec{spec("default", "draft-gray")}, []string{"demo-*"}))
}

func TestFilterTestResultsNarrowsResourceSpecs(t *testing.T) {
	spec := func(name string) v1alpha1.TestResourceSpec {
		return v1alpha1.TestResourceSpec{Version: "v1", Kind: "ConfigMap", Namespace: "default", Name: name}
	}
	pass := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "require-catalog-team",
			Kind:   "ConfigMap",
			Result: openreportsv1alpha1.Result("pass"),
		},
		TestResultData: v1alpha1.TestResultData{
			ResourceSpecs: []v1alpha1.TestResourceSpec{spec("demo-blue"), spec("draft-gray")},
		},
	}

	selector, errs := filter.ParseFilter("policy=*,rule=*,resource=demo-*")
	require.Empty(t, errs)
	filtered := filterTestResults([]v1alpha1.TestResult{pass}, selector, filter.ExtractResourceFilters("policy=*,rule=*,resource=demo-*"))
	require.Len(t, filtered, 1)
	assert.Equal(t, []v1alpha1.TestResourceSpec{spec("demo-blue")}, filtered[0].ResourceSpecs)

	selector, errs = filter.ParseFilter("policy=*,rule=*,resource=missing-*")
	require.Empty(t, errs)
	assert.Empty(t, filterTestResults([]v1alpha1.TestResult{pass}, selector, filter.ExtractResourceFilters("policy=*,rule=*,resource=missing-*")))
}

func TestFilterTestResultsSpecsMatchEveryResourcePattern(t *testing.T) {
	spec := func(name string) v1alpha1.TestResourceSpec {
		return v1alpha1.TestResourceSpec{Version: "v1", Kind: "ConfigMap", Namespace: "default", Name: name}
	}
	// the sub filters are combined with AND, as for declared resources: a result
	// that only declares specs has to match every pattern as well
	onlyDemo := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{Policy: "require-catalog-team", Kind: "ConfigMap"},
		TestResultData: v1alpha1.TestResultData{ResourceSpecs: []v1alpha1.TestResourceSpec{spec("demo-blue")}},
	}
	both := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{Policy: "require-catalog-team", Kind: "ConfigMap"},
		TestResultData: v1alpha1.TestResultData{ResourceSpecs: []v1alpha1.TestResourceSpec{spec("demo-blue"), spec("draft-gray")}},
	}

	selector, errs := filter.ParseFilter("policy=*,rule=*,resource=demo-*,resource=draft-*")
	require.Empty(t, errs)
	filtered := filterTestResults([]v1alpha1.TestResult{onlyDemo, both}, selector,
		filter.ExtractResourceFilters("policy=*,rule=*,resource=demo-*,resource=draft-*"))
	require.Len(t, filtered, 1)
	assert.Equal(t, []v1alpha1.TestResourceSpec{spec("demo-blue"), spec("draft-gray")}, filtered[0].ResourceSpecs)
}
