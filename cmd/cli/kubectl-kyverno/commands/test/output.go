package test

import (
	"context"
	"fmt"
	"io"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno-json/pkg/engine/assert"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/runtime"
)

func printCheckResult(
	checks []v1alpha1.CheckResult,
	responses []engineapi.EngineResponse,
	rc *resultCounts,
	resultsTable *table.Table,
) error {
	ctx := context.Background()
	testCount := 1
	for _, check := range checks {
		// filter engine responses
		matchingEngineResponses := responses
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
					"exception ":        rule.Exception(),
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

func printTestResult(
	tests []v1alpha1.TestResult,
	responses []engineapi.EngineResponse,
	rc *resultCounts,
	resultsTable *table.Table,
	fs billy.Filesystem,
	resoucePath string,
) error {
	testCount := 1
	for _, test := range tests {
		// lookup matching engine responses (without the resource name)
		// to reduce the search scope
		responses := lookupEngineResponses(test, "", responses...)
		// TODO fix deprecated fields
		// identify the resources to be looked up
		var resources []string
		if test.Resources != nil {
			resources = append(resources, test.Resources...)
		} else if test.Resource != "" {
			resources = append(resources, test.Resource)
		}
		for _, resource := range resources {
			var rows []table.Row
			// lookup matching engine responses (with the resource name this time)
			for _, response := range lookupEngineResponses(test, resource, responses...) {
				// lookup matching rule responses
				for _, rule := range lookupRuleResponses(test, response.PolicyResponse.Rules...) {
					// perform test checks
					ok, message, reason := checkResult(test, fs, resoucePath, response, rule)
					// if checks failed but we were expecting a fail it's considered a success
					success := ok || (!ok && test.Result == policyreportv1alpha2.StatusFail)
					row := table.Row{
						RowCompact: table.RowCompact{
							ID:        testCount,
							Policy:    color.Policy("", test.Policy),
							Rule:      color.Rule(test.Rule),
							Resource:  color.Resource(test.Kind, test.Namespace, resource),
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
					testCount++
					rows = append(rows, row)
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
			// if not found
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

func printFailedTestResult(out io.Writer, resultsTable table.Table, detailedResults bool) {
	printer := table.NewTablePrinter(out)
	for i := range resultsTable.RawRows {
		resultsTable.RawRows[i].ID = i + 1
	}
	fmt.Fprintf(out, "Aggregated Failed Test Cases : ")
	fmt.Fprintln(out)
	printer.Print(resultsTable.Rows(detailedResults))
}
