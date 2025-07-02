package fix

import (
	"cmp"
	"slices"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func FixTest(test v1alpha1.Test, compress bool) (v1alpha1.Test, []string, error) {
	var messages []string
	if test.APIVersion == "" {
		messages = append(messages, "api version is not set, setting `cli.kyverno.io/v1alpha1`")
		test.APIVersion = "cli.kyverno.io/v1alpha1"
	}
	if test.Kind == "" {
		messages = append(messages, "kind is not set, setting `Test`")
		test.Kind = "Test"
	}
	if test.Name != "" {
		messages = append(messages, "name is deprecated, moving it into `metadata.name`")
		test.ObjectMeta.Name = test.Name
		test.Name = ""
	}
	if len(test.Policies) == 0 {
		messages = append(messages, "test has no policies")
	}
	if len(test.Resources) == 0 {
		messages = append(messages, "test has no resources")
	}
	results := make([]v1alpha1.TestResult, 0, len(test.Results))
	for _, result := range test.Results {
		unique := sets.New(result.Resources...)
		if len(result.Resources) != len(unique) {
			messages = append(messages, "test results contains duplicate resources")
			result.Resources = unique.UnsortedList()
		}
		results = append(results, result)
	}
	if compress {
		compressed := map[v1alpha1.TestResultBase]v1alpha1.TestResultData{}
		for _, result := range results {
			data := compressed[result.TestResultBase]
			data.Resources = append(data.Resources, result.Resources...)
			data.ResourceSpecs = append(data.ResourceSpecs, result.ResourceSpecs...)
			compressed[result.TestResultBase] = data
		}
		results = nil
		for k, v := range compressed {
			unique := sets.New(v.Resources...)
			if len(v.Resources) != len(unique) {
				messages = append(messages, "test results contains duplicate resources")
				v.Resources = unique.UnsortedList()
			}
			results = append(results, v1alpha1.TestResult{
				TestResultBase: k,
				TestResultData: v,
			})
		}
	}
	slices.SortFunc(results, func(a, b v1alpha1.TestResult) int {
		if x := cmp.Compare(a.Policy, b.Policy); x != 0 {
			return x
		}
		if x := cmp.Compare(a.Rule, b.Rule); x != 0 {
			return x
		}
		if x := cmp.Compare(a.Result, b.Result); x != 0 {
			return x
		}
		if x := cmp.Compare(a.Kind, b.Kind); x != 0 {
			return x
		}
		if x := cmp.Compare(a.GeneratedResource, b.GeneratedResource); x != 0 {
			return x
		}
		if x := cmp.Compare(a.CloneSourceResource, b.CloneSourceResource); x != 0 {
			return x
		}
		slices.Sort(a.Resources)
		slices.Sort(b.Resources)
		if x := cmp.Compare(len(a.Resources), len(b.Resources)); x != 0 {
			return x
		}
		if len(a.Resources) == len(b.Resources) {
			for i := range a.Resources {
				if x := cmp.Compare(a.Resources[i], b.Resources[i]); x != 0 {
					return x
				}
			}
		}
		// TODO resource specs
		return 0
	})
	test.Results = results
	return test, messages, nil
}
