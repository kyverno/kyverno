package fix

import (
	"errors"
	"fmt"

	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/test"
	"golang.org/x/exp/slices"
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
			results = append(results, testapi.TestResult{
				TestResultBase: k,
				Resources:      v,
			})
		}
	}
	slices.SortFunc(results, func(a, b testapi.TestResult) bool {
		if a.Policy < b.Policy {
			return true
		}
		if a.Rule < b.Rule {
			return true
		}
		if a.Result < b.Result {
			return true
		}
		if a.Kind < b.Kind {
			return true
		}
		if a.PatchedResource < b.PatchedResource {
			return true
		}
		if a.GeneratedResource < b.GeneratedResource {
			return true
		}
		if a.CloneSourceResource < b.CloneSourceResource {
			return true
		}
		slices.Sort(a.Resources)
		slices.Sort(b.Resources)
		if len(a.Resources) < len(b.Resources) {
			return true
		}
		if len(a.Resources) == len(b.Resources) {
			for i := range a.Resources {
				if a.Resources[i] < b.Resources[i] {
					return true
				}
			}
		}
		return false
	})
	test.Results = results
	return test, messages, nil
}
