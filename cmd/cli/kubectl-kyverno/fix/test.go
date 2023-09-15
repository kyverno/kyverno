package fix

import (
	"errors"
	"fmt"

	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/test"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"
)

func FixTest(test testapi.Test, compress bool) (testapi.Test, []string, error) {
	var messages []string
	if test.Name == "" {
		messages = append(messages, "name is not set")
	}
	if len(test.Policies) == 0 {
		messages = append(messages, "test has no policies")
	}
	if len(test.Resources) == 0 {
		messages = append(messages, "test has no resources")
	}
	var results []testapi.TestResult
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
		compressed := map[testapi.TestResultBase][]string{}
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
			results = append(results, testapi.TestResult{
				TestResultBase: k,
				Resources:      v,
			})
		}
	}
	slices.SortFunc(results, func(a, b testapi.TestResult) int {
		if x := datautils.Compare(a.Policy, b.Policy); x != 0 {
			return x
		}
		if x := datautils.Compare(a.Rule, b.Rule); x != 0 {
			return x
		}
		if x := datautils.Compare(a.Result, b.Result); x != 0 {
			return x
		}
		if x := datautils.Compare(a.Kind, b.Kind); x != 0 {
			return x
		}
		if x := datautils.Compare(a.PatchedResource, b.PatchedResource); x != 0 {
			return x
		}
		if x := datautils.Compare(a.GeneratedResource, b.GeneratedResource); x != 0 {
			return x
		}
		if x := datautils.Compare(a.CloneSourceResource, b.CloneSourceResource); x != 0 {
			return x
		}
		slices.Sort(a.Resources)
		slices.Sort(b.Resources)
		if x := datautils.Compare(len(a.Resources), len(b.Resources)); x != 0 {
			return x
		}
		if len(a.Resources) == len(b.Resources) {
			for i := range a.Resources {
				if x := datautils.Compare(a.Resources[i], b.Resources[i]); x != 0 {
					return x
				}
			}
		}
		return 0
	})
	test.Results = results
	return test, messages, nil
}
