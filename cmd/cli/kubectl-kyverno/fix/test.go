package fix

import (
	"cmp"
	"errors"
	"fmt"
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
	var results []v1alpha1.TestResult
	for _, result := range test.Results {
		if result.Resource != "" && len(result.Resources) != 0 {
			messages = append(messages, "test result should not use both `resource` and `resources` fields")
		}
		if result.Resource != "" {
			var resources []string
			messages = append(messages, "test result uses deprecated `resource` field, moving it into the `resources` field")
			resources = append(resources, result.Resources...)
			resources = append(resources, result.Resource)
			result.Resources = resources
			result.Resource = ""
		}
		unique := sets.New(result.Resources...)
		if len(result.Resources) != len(unique) {
			messages = append(messages, "test results contains duplicate resources")
			result.Resources = unique.UnsortedList()
		}
		if result.Namespace != "" {
			messages = append(messages, "test result uses deprecated `namespace` field, replacing `policy` with a `<namespace>/<name>` pattern")
			result.Policy = fmt.Sprintf("%s/%s", result.Namespace, result.Policy)
			result.Namespace = ""
		}
		if result.Status != "" && result.Result != "" {
			return test, messages, errors.New("test result should not use both `status` and `result` fields")
		}
		if result.Status != "" && result.Result == "" {
			messages = append(messages, "test result uses deprecated `status` field, moving it into the `result` field")
			result.Result = result.Status
			result.Status = ""
		}
		results = append(results, result)
	}
	if compress {
		compressed := map[v1alpha1.TestResultBase][]string{}
		for _, result := range results {
			compressed[result.TestResultBase] = append(compressed[result.TestResultBase], result.Resources...)
		}
		results = nil
		for k, v := range compressed {
			unique := sets.New(v...)
			if len(v) != len(unique) {
				messages = append(messages, "test results contains duplicate resources")
				v = unique.UnsortedList()
			}
			results = append(results, v1alpha1.TestResult{
				TestResultBase: k,
				Resources:      v,
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
		if x := cmp.Compare(a.PatchedResource, b.PatchedResource); x != 0 {
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
		return 0
	})
	test.Results = results
	return test, messages, nil
}
