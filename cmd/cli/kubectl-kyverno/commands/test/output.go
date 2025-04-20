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
				data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(response.Policy().AsObject())
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
			for _, r := range test.Resources {
				for _, m := range []map[string][]engineapi.EngineResponse{responses.Target, responses.Trigger} {
					for resourceGVKAndName := range m {
						nameParts := strings.Split(resourceGVKAndName, ",")
						nsAndName := strings.Split(r, "/")
						if len(nsAndName) == 1 {
							if r == nameParts[len(nameParts)-1] {
								resources = append(resources, resourceGVKAndName)
							}
						}
						if len(nsAndName) == 2 {
							if nsAndName[0] == nameParts[len(nameParts)-2] && nsAndName[1] == nameParts[len(nameParts)-1] {
								resources = append(resources, resourceGVKAndName)
							}
						}
					}
				}
			}
			for _, resourceSpec := range test.ResourceSpecs {
				for _, m := range []map[string][]engineapi.EngineResponse{responses.Target, responses.Trigger} {
					for resourceGVKAndName := range m {
						nameParts := strings.Split(resourceGVKAndName, ",")
						if resourceSpec.Group == "" {
							if resourceSpec.Version != nameParts[0] {
								continue
							}
						} else {
							if resourceSpec.Group+"/"+resourceSpec.Version != nameParts[0] {
								continue
							}
						}
						if resourceSpec.Namespace != nameParts[len(nameParts)-2] {
							continue
						}
						if resourceSpec.Name == nameParts[len(nameParts)-1] {
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
			var resourceSkipped bool
			if _, ok := responses.Trigger[resource]; ok {
				for _, response := range responses.Trigger[resource] {
					polNameNs := strings.Split(test.Policy, "/")
					if response.Policy().GetName() != polNameNs[len(polNameNs)-1] {
						continue
					}
					for _, rule := range lookupRuleResponses(test, response.PolicyResponse.Rules...) {
						r := response.Resource

						if test.IsValidatingAdmissionPolicy || test.IsValidatingPolicy || test.IsImageValidatingPolicy {
							ok, message, reason := checkResult(test, fs, resoucePath, response, rule, r)
							if strings.Contains(message, "not found in manifest") {
								resourceSkipped = true
								continue
							}

							resourceRows := createRowsAccordingToResults(test, rc, &testCount, ok, message, reason, strings.Replace(resource, ",", "/", -1), rule)
							rows = append(rows, resourceRows...)
							continue
						}

						if rule.RuleType() != "Generation" {
							if rule.RuleType() == "Mutation" {
								r = response.PatchedResource
							}

							ok, message, reason := checkResult(test, fs, resoucePath, response, rule, r)
							if strings.Contains(message, "not found in manifest") {
								resourceSkipped = true
								continue
							}

							success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
							resourceRows := createRowsAccordingToResults(test, rc, &testCount, success, message, reason, strings.Replace(resource, ",", "/", -1), rule)
							rows = append(rows, resourceRows...)
						} else {
							generatedResources := rule.GeneratedResources()
							for _, r := range generatedResources {
								ok, message, reason := checkResult(test, fs, resoucePath, response, rule, *r)

								success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
								resourceRows := createRowsAccordingToResults(test, rc, &testCount, success, message, reason, r.GetName(), rule)
								rows = append(rows, resourceRows...)
							}
						}
					}

					// if there are no RuleResponse, the resource has been excluded. This is a pass.
					if len(rows) == 0 && !resourceSkipped {
						row := table.Row{
							RowCompact: table.RowCompact{
								ID:        testCount,
								Policy:    color.Policy("", test.Policy),
								Rule:      color.Rule(test.Rule),
								Resource:  color.Resource(test.Kind, test.Namespace, strings.Replace(resource, ",", "/", -1)),
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
					nameParts := strings.Split(resource, ",")
					name, ns, kind, apiVersion := nameParts[len(nameParts)-1], nameParts[len(nameParts)-2], nameParts[len(nameParts)-3], nameParts[len(nameParts)-4]

					r, rule := extractPatchedTargetFromEngineResponse(apiVersion, kind, name, ns, response)
					ok, message, reason := checkResult(test, fs, resoucePath, response, *rule, *r)

					success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
					resourceRows := createRowsAccordingToResults(test, rc, &testCount, success, message, reason, strings.Replace(resource, ",", "/", -1), *rule)
					rows = append(rows, resourceRows...)
				}
			}

			if len(rows) == 0 && !resourceSkipped {
				row := table.Row{
					RowCompact: table.RowCompact{
						ID:        testCount,
						Policy:    color.Policy("", test.Policy),
						Rule:      color.Rule(test.Rule),
						Resource:  color.Resource(test.Kind, test.Namespace, strings.Replace(resource, ",", "/", -1)),
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

func createRowsAccordingToResults(test v1alpha1.TestResult, rc *resultCounts, globalTestCounter *int, success bool, message string, reason string, resourceGVKAndName string, rule engineapi.RuleResponse) []table.Row {
	resourceParts := strings.Split(resourceGVKAndName, "/")
	rows := []table.Row{}
	row := table.Row{
		RowCompact: table.RowCompact{
			ID:        *globalTestCounter,
			Policy:    color.Policy("", test.Policy),
			Rule:      color.Rule(test.Rule),
			Resource:  color.Resource(strings.Join(resourceParts[:len(resourceParts)-1], "/"), test.Namespace, resourceParts[len(resourceParts)-1]),
			Reason:    reason,
			IsFailure: !success,
		},
		Message: message,
	}
	if test.Result == policyreportv1alpha2.StatusError && rule.Status() == engineapi.RuleStatusError {
		row.Result = color.ResultPass()
		row.Reason = "Expected error occurred"
		rc.Pass++
	} else if test.Result == policyreportv1alpha2.StatusFail && rule.Status() == engineapi.RuleStatusFail {
		row.Result = color.ResultPass()
		row.Reason = "Expected failure occurred"
		rc.Pass++
	} else if test.Result == policyreportv1alpha2.StatusPass && rule.Status() == engineapi.RuleStatusPass {
		row.Result = color.ResultPass()
		row.Reason = "Expected success occurred"
		rc.Pass++
	} else if rule.Status() == engineapi.RuleStatusSkip {
		row.Result = color.ResultSkip()
		row.Reason = "Rule was skipped"
		rc.Skip++
	} else {
		row.Result = color.ResultFail()
		row.Reason = fmt.Sprintf("Expected %s, but got %s", test.Result, rule.Status())
		rc.Fail++
	}

	*globalTestCounter++
	rows = append(rows, row)

	// if there are no RuleResponse, the resource has been excluded. This is a pass.
	if len(rows) == 0 {
		row := table.Row{
			RowCompact: table.RowCompact{
				ID:        *globalTestCounter,
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
		*globalTestCounter++
		rows = append(rows, row)
	}
	return rows
}

func extractPatchedTargetFromEngineResponse(apiVersion, kind, resourceName, resourceNamespace string, response engineapi.EngineResponse) (*unstructured.Unstructured, *engineapi.RuleResponse) {
	for _, rule := range response.PolicyResponse.Rules {
		r, _, _ := rule.PatchedTarget()
		if r != nil {
			if resourceNamespace == "" {
				resourceNamespace = r.GetNamespace()
			}
			if r.GetAPIVersion() == apiVersion && r.GetKind() == kind && r.GetName() == resourceName && r.GetNamespace() == resourceNamespace {
				return r, &rule
			}
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
