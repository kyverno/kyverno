package test

import (
	"fmt"
	"io"

	"github.com/go-git/go-billy/v5"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func printTestResult(
	out io.Writer,
	tests []v1alpha1.TestResult,
	responses []engineapi.EngineResponse,
	rc *resultCounts,
	failOnly bool,
	detailedResults bool,
	fs billy.Filesystem,
	resoucePath string,
) (table.Table, error) {
	printer := table.NewTablePrinter(out)
	var resultsTable table.Table
	var countDeprecatedResource int
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
			countDeprecatedResource++
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
	fmt.Fprintln(out)
	printer.Print(resultsTable.Rows(detailedResults))
	fmt.Fprintln(out)
	return resultsTable, nil
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
