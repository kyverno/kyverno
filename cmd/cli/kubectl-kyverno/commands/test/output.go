package test

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno-json/pkg/engine/assert"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func printCheckResult(
	checks []v1alpha1.CheckResult,
	responses TestResponse,
	rc *resultCounts,
	resultsTable *table.Table,
) error {
	ctx := context.Background()
	testCount := 1
	for _, check := range checks {
		// filter engine responses
		var matchingEngineResponses []engineapi.EngineResponse
		for _, engineresponses := range responses.Trigger {
			matchingEngineResponses = append(matchingEngineResponses, engineresponses...)
		}
		// 1. by resource
		if check.Match.Resource != nil {
			var filtered []engineapi.EngineResponse
			for _, response := range matchingEngineResponses {
				errs, err := assert.Assert(ctx, nil, assert.Parse(ctx, check.Match.Resource.Value), response.Resource.UnstructuredContent(), nil)
				if err != nil {
					return err
				}
				if len(errs) == 0 {
					filtered = append(filtered, response)
				}
			}
			matchingEngineResponses = filtered
		}
		// 2. by policy
		if check.Match.Policy != nil {
			var filtered []engineapi.EngineResponse
			for _, response := range matchingEngineResponses {
				data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(response.Policy().MetaObject())
				if err != nil {
					return err
				}
				errs, err := assert.Assert(ctx, nil, assert.Parse(ctx, check.Match.Policy.Value), data, nil)
				if err != nil {
					return err
				}
				if len(errs) == 0 {
					filtered = append(filtered, response)
				}
			}
			matchingEngineResponses = filtered
		}
		for _, response := range matchingEngineResponses {
			// filter rule responses
			matchingRuleResponses := response.PolicyResponse.Rules
			if check.Match.Rule != nil {
				var filtered []engineapi.RuleResponse
				for _, response := range matchingRuleResponses {
					data := map[string]any{
						"name": response.Name(),
					}
					errs, err := assert.Assert(ctx, nil, assert.Parse(ctx, check.Match.Rule.Value), data, nil)
					if err != nil {
						return err
					}
					if len(errs) == 0 {
						filtered = append(filtered, response)
					}
				}
				matchingRuleResponses = filtered
			}
			for _, rule := range matchingRuleResponses {
				// perform check
				data := map[string]any{
					"name":     rule.Name(),
					"ruleType": rule.RuleType(),
					"message":  rule.Message(),
					"status":   string(rule.Status()),
					// generatedResource unstructured.Unstructured
					// patchedTarget *unstructured.Unstructured
					// patchedTargetParentResourceGVR metav1.GroupVersionResource
					// patchedTargetSubresourceName string
					// podSecurityChecks contains pod security checks (only if this is a pod security rule)
					"podSecurityChecks": rule.PodSecurityChecks(),
					"exceptions":        rule.Exceptions(),
				}
				if check.Assert.Value != nil {
					errs, err := assert.Assert(ctx, nil, assert.Parse(ctx, check.Assert.Value), data, nil)
					if err != nil {
						return err
					}
					row := table.Row{
						RowCompact: table.RowCompact{
							ID:        testCount,
							Policy:    color.Policy("", response.Policy().GetName()),
							Rule:      color.Rule(rule.Name()),
							Resource:  color.Resource(response.Resource.GetKind(), response.Resource.GetNamespace(), response.Resource.GetName()),
							IsFailure: len(errs) != 0,
						},
						Message: rule.Message(),
					}
					if len(errs) == 0 {
						row.Result = color.ResultPass()
						row.Reason = "Ok"
						if rule.Status() == engineapi.RuleStatusSkip {
							rc.Skip++
						} else {
							rc.Pass++
						}
					} else {
						row.Result = color.ResultFail()
						row.Reason = errs.ToAggregate().Error()
						rc.Fail++
					}
					resultsTable.Add(row)
					testCount++
				}
				if check.Error.Value != nil {
					errs, err := assert.Assert(ctx, nil, assert.Parse(ctx, check.Error.Value), data, nil)
					if err != nil {
						return err
					}
					row := table.Row{
						RowCompact: table.RowCompact{
							ID:        testCount,
							Policy:    color.Policy("", response.Policy().GetName()),
							Rule:      color.Rule(rule.Name()),
							Resource:  color.Resource(response.Resource.GetKind(), response.Resource.GetNamespace(), response.Resource.GetName()),
							IsFailure: len(errs) != 0,
						},
						Message: rule.Message(),
					}
					if len(errs) != 0 {
						row.Result = color.ResultPass()
						row.Reason = errs.ToAggregate().Error()
						if rule.Status() == engineapi.RuleStatusSkip {
							rc.Skip++
						} else {
							rc.Pass++
						}
					} else {
						row.Result = color.ResultFail()
						row.Reason = "The assertion succeeded but was expected to fail"
						rc.Fail++
					}
					resultsTable.Add(row)
					testCount++
				}
			}
		}
	}
	return nil
}

// a test that contains a policy that may contain several rules
func printTestResult(
	tests []v1alpha1.TestResult,
	responses *TestResponse,
	rc *resultCounts,
	resultsTable *table.Table,
	fs billy.Filesystem,
	resoucePath string,
) error {
	testCount := 1
	for _, test := range tests {
		var resources []string
		// The test specifies certain resources to check, results will be checked for those resources only
		if test.Resource != "" {
			test.Resources = append(test.Resources, test.Resource)
		}
		if test.Resources != nil {
			// this matching logic will be removed once resources become an array of gvk/name
			// the problem with this is that it will match resources with the same name but different kinds
			for _, r := range test.Resources {
				for _, m := range []map[string][]engineapi.EngineResponse{responses.Target, responses.Trigger} {
					for resourceGVKAndName := range m {
						nameParts := strings.Split(resourceGVKAndName, "/")
						if nameParts[len(nameParts)-1] == r {
							resources = append(resources, resourceGVKAndName)
						}
					}
				}
			}
		}

		// The test specifies no resources, check all results
		if len(resources) == 0 {
			for r := range responses.Target {
				resources = append(resources, r)
			}
			for r := range responses.Trigger {
				resources = append(resources, r)
			}
		}

		for _, resource := range resources {
			var rows []table.Row
			if _, ok := responses.Trigger[resource]; ok {
				for _, response := range responses.Trigger[resource] {
					for _, rule := range lookupRuleResponses(test, response.PolicyResponse.Rules...) {
						r := response.Resource

						if rule.RuleType() != "Generation" {
							if rule.RuleType() == "Mutation" {
								r = response.PatchedResource
							}
							nameParts := strings.Split(resource, "/")
							ok, message, reason := checkResult(test, fs, resoucePath, response, rule, r, nameParts[len(nameParts)-1])

							success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
							resourceRows := createRowsAccordingToResults(test, rc, testCount, success, message, reason, resource)
							rows = append(rows, resourceRows...)
						}

						generatedResources := rule.GeneratedResources()
						for _, r := range generatedResources {
							nameParts := strings.Split(resource, "/")
							ok, message, reason := checkResult(test, fs, resoucePath, response, rule, *r, nameParts[len(nameParts)-1])

							success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
							resourceRows := createRowsAccordingToResults(test, rc, testCount, success, message, reason, resource)
							rows = append(rows, resourceRows...)
						}
					}

					// if there are no RuleResponse, the resource has been excluded. This is a pass.
					if len(rows) == 0 {
						row := table.Row{
							RowCompact: table.RowCompact{
								ID:        testCount,
								Policy:    color.Policy("", test.Policy),
								Rule:      color.Rule(test.Rule),
								Resource:  color.Resource(test.Kind, test.Namespace, resource),
								Result:    color.ResultPass(),
								Reason:    color.Excluded(),
								IsFailure: false,
							},
							Message: color.Excluded(),
						}
						rc.Skip++
						testCount++
						rows = append(rows, row)
					}
				}
			}

			// Check if the resource specified exists in the targets
			if _, ok := responses.Target[resource]; ok {
				for _, response := range responses.Target[resource] {
					// we are doing this twice which is kinda not nice

					nameParts := strings.Split(resource, "/")

					r, rule := extractPatchedTargetFromEngineResponse(nameParts[len(nameParts)-1], response)
					ok, message, reason := checkResult(test, fs, resoucePath, response, *rule, *r, nameParts[len(nameParts)-1])

					success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
					resourceRows := createRowsAccordingToResults(test, rc, testCount, success, message, reason, resource)
					rows = append(rows, resourceRows...)
				}
			}

			if len(rows) == 0 {
				row := table.Row{
					RowCompact: table.RowCompact{
						ID:        testCount,
						Policy:    color.Policy("", test.Policy),
						Rule:      color.Rule(test.Rule),
						Resource:  color.Resource(test.Kind, test.Namespace, resource),
						IsFailure: true,
						Result:    color.ResultFail(),
						Reason:    color.NotFound(),
					},
					Message: color.NotFound(),
				}
				testCount++
				resultsTable.Add(row)
				rc.Fail++
			} else {
				resultsTable.Add(rows...)
			}
		}
	}
	return nil
}

func createRowsAccordingToResults(test v1alpha1.TestResult, rc *resultCounts, globalTestCounter int, success bool, message string, reason string, resourceGVKAndName string) []table.Row {
	resourceParts := strings.Split(resourceGVKAndName, "/")
	rows := []table.Row{}
	row := table.Row{
		RowCompact: table.RowCompact{
			ID:        globalTestCounter,
			Policy:    color.Policy("", test.Policy),
			Rule:      color.Rule(test.Rule),
			Resource:  color.Resource(strings.Join(resourceParts[:len(resourceParts)-1], "/"), test.Namespace, resourceParts[len(resourceParts)-1]),
			Reason:    reason,
			IsFailure: !success,
		},
		Message: message,
	}
	if success {
		row.Result = color.ResultPass()
		if test.Result == policyreportv1alpha2.StatusSkip {
			rc.Skip++
		} else {
			rc.Pass++
		}
	} else {
		row.Result = color.ResultFail()
		rc.Fail++
	}
	globalTestCounter++
	rows = append(rows, row)

	// if there are no RuleResponse, the resource has been excluded. This is a pass.
	if len(rows) == 0 {
		row := table.Row{
			RowCompact: table.RowCompact{
				ID:        globalTestCounter,
				Policy:    color.Policy("", test.Policy),
				Rule:      color.Rule(test.Rule),
				Resource:  color.Resource(strings.Join(resourceParts[:len(resourceParts)-1], "/"), test.Namespace, resourceParts[len(resourceParts)-1]), // todo: handle namespace
				Result:    color.ResultPass(),
				Reason:    color.Excluded(),
				IsFailure: false,
			},
			Message: color.Excluded(),
		}
		rc.Skip++
		globalTestCounter++
		rows = append(rows, row)
	}
	return rows
}

func extractPatchedTargetFromEngineResponse(resourceName string, response engineapi.EngineResponse) (*unstructured.Unstructured, *engineapi.RuleResponse) {
	for _, rule := range response.PolicyResponse.Rules {
		if r, _, _ := rule.PatchedTarget(); r.GetName() == resourceName {
			return r, &rule
		}
	}
	return nil, nil
}

func printFailedTestResult(out io.Writer, resultsTable table.Table, detailedResults bool) {
	printer := table.NewTablePrinter(out)
	for i := range resultsTable.RawRows {
		resultsTable.RawRows[i].ID = i + 1
	}
	fmt.Fprintf(out, "Aggregated Failed Test Cases : ")
	fmt.Fprintln(out)
	printer.Print(resultsTable.Rows(detailedResults))
}
