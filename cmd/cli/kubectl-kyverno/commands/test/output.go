package test

import (
	"context"
	"encoding/json"
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
	"gopkg.in/yaml.v3"
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
		}
		// If no resource is specified, check all resources present in the engine responses
		if resources == nil {
			var foundResources []string
			for _, m := range []map[string][]engineapi.EngineResponse{responses.Target, responses.Trigger} {
				for resourceGVKAndName := range m {
					foundResources = append(foundResources, resourceGVKAndName)
				}
			}
			if len(foundResources) > 0 {
				resources = foundResources
			}
		}
		var rows []table.Row
		resourceSkipped := false
		for _, resource := range resources {
			var engineResponses []engineapi.EngineResponse
			// Check if the resource is a target or a trigger or both
			match := false
			for resourceType, resourceInfos := range map[string]map[string][]engineapi.EngineResponse{
				"Target":  responses.Target,
				"Trigger": responses.Trigger,
			} {
				var ok bool
				var resourceResponses []engineapi.EngineResponse
				if resourceResponses, ok = resourceInfos[resource]; !ok {
					continue
				}
				match = true

				// We don't need to use these variables, but keeping the logic to extract them
				// for potential future use
				if resourceType == "Target" {
					resourceSkipped = true
					continue
				}
				if resourceType == "Trigger" {
					resourceSkipped = true
					continue
				}
				engineResponses = append(engineResponses, resourceResponses...)
			}
			if !match {
				continue
			}
			for _, response := range engineResponses {
				var ruleResponses []engineapi.RuleResponse
				if test.Rule != "" {
					ruleResponses = lookupRuleResponses(test, response.PolicyResponse.Rules...)
				} else {
					ruleResponses = response.PolicyResponse.Rules
				}
				if len(ruleResponses) == 0 {
					continue
				}
				if len(ruleResponses) == 1 {
					rule := ruleResponses[0]
					// Check if a particular rule response has generated resources
					if test.GeneratedResource != "" || test.PatchedResource != "" {
						generatedResources := rule.GeneratedResources()
						if len(generatedResources) == 0 {
							continue
						}
						if test.Policy != "" && response.Policy().GetName() != test.Policy {
							continue
						}

						ok, message, reason := checkResult(test, fs, resoucePath, response, rule, *generatedResources[0])
						success := (ok && test.Result == policyreportv1alpha2.StatusPass) || (!ok && test.Result == policyreportv1alpha2.StatusFail)
						resourceRows := createRowsAccordingToResults(test, rc, &testCount, success, message, reason, strings.Replace(resource, ",", "/", -1))
						rows = append(rows, resourceRows...)
					} else {
						if test.Policy != "" && response.Policy().GetName() != test.Policy {
							continue
						}

						ok, message, reason := checkResult(test, fs, resoucePath, response, rule, response.Resource)
						success := (ok && test.Result == policyreportv1alpha2.StatusPass) || (!ok && test.Result == policyreportv1alpha2.StatusFail)
						resourceRows := createRowsAccordingToResults(test, rc, &testCount, success, message, reason, strings.Replace(resource, ",", "/", -1))
						rows = append(rows, resourceRows...)
					}
				} else {
					// Check rule which has generated multiple resources
					nameParts := strings.Split(resource, ",")
					name, ns, kind, apiVersion := nameParts[len(nameParts)-1], nameParts[len(nameParts)-2], nameParts[len(nameParts)-3], nameParts[len(nameParts)-4]

					r, rule := extractPatchedTargetFromEngineResponse(apiVersion, kind, name, ns, response)
					ok, message, reason := checkResult(test, fs, resoucePath, response, *rule, *r)

					success := (ok && test.Result == policyreportv1alpha2.StatusPass) || (!ok && test.Result == policyreportv1alpha2.StatusFail)
					resourceRows := createRowsAccordingToResults(test, rc, &testCount, success, message, reason, strings.Replace(resource, ",", "/", -1))
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

func createRowsAccordingToResults(test v1alpha1.TestResult, rc *resultCounts, globalTestCounter *int, success bool, message string, reason string, resourceGVKAndName string) []table.Row {
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

func printOutputFormats(out io.Writer, outputFormat string, resultTable table.Table, detailedResults bool) {
	output := make([]interface{}, 0, len(resultTable.RawRows))
	failedTests := 0
	for _, row := range resultTable.RawRows {
		// Only count actual failures, not expected failures
		if row.IsFailure {
			failedTests++
		}

		rowMap := map[string]interface{}{
			"ID":       row.ID,
			"POLICY":   row.Policy,
			"RULE":     row.Rule,
			"RESOURCE": row.Resource,
			"RESULT":   row.Result,
			"REASON":   row.Reason,
		}
		if detailedResults {
			rowMap["Message"] = row.Message
		}
		output = append(output, rowMap)
	}
	var finalOutput []byte
	if outputFormat == "markdown" {
		var b strings.Builder
		headers := []string{"ID", "POLICY", "RULE", "RESOURCE", "RESULT", "REASON"}
		if detailedResults {
			headers = append(headers, "MESSAGE")
		}
		b.WriteString("| " + strings.Join(headers, " | ") + " | \n")
		b.WriteString("|" + strings.Repeat("----|", len(headers)) + "\n")
		for _, row := range resultTable.RawRows {
			b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |",
				row.ID, row.Policy, row.Rule, row.Resource, row.Result, row.Reason))
			if detailedResults {
				b.WriteString(fmt.Sprintf(" %s |\n", row.Message))
			} else {
				b.WriteString("\n")
			}
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, b.String())
		fmt.Fprintln(out)
	} else if outputFormat == "junit" {
		var b strings.Builder
		b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		b.WriteString(fmt.Sprintf("<testsuites tests=\"%d\" failures=\"%d\">\n", len(output), failedTests))
		suites := make(map[string][]table.Row)
		for _, row := range resultTable.RawRows {
			suites[row.Policy] = append(suites[row.Policy], row)
		}
		for policyName, rows := range suites {
			failures := 0
			for _, row := range rows {
				if row.IsFailure {
					failures++
				}
			}
			b.WriteString(fmt.Sprintf(" <testsuite name=\"%s\" tests=\"%d\" failures=\"%d\">\n", policyName, len(rows), failures))
			for _, policyRow := range rows {
				b.WriteString(fmt.Sprintf("  <testcase classname=\"%s\" name=\"%s\">\n", policyRow.Rule, policyRow.Resource))
				if policyRow.IsFailure {
					b.WriteString(fmt.Sprintf("   <failure message=\"%s\">\n    Policy: %s\n    Rule: %s\n    Resource: %s\n    Result: %s", policyRow.Reason, policyRow.Policy, policyRow.Rule, policyRow.Resource, policyRow.Result))
					if detailedResults {
						b.WriteString(fmt.Sprintf("    Message: %s\n   </failure>\n", policyRow.Message))
					} else {
						b.WriteString("\n   </failure>\n")
					}
				} else {
					b.WriteString(fmt.Sprintf("   <system-out><![CDATA[\n    Reason: %s\n    Policy: %s\n    Rule: %s\n    Resource: %s\n", policyRow.Reason, policyRow.Policy, policyRow.Rule, policyRow.Resource))
					if detailedResults {
						b.WriteString(fmt.Sprintf("    Message: %s\n   ]]></system-out>\n", policyRow.Message))
					} else {
						b.WriteString("   ]]></system-out>\n")
					}
				}
				b.WriteString("  </testcase>\n")
			}
			b.WriteString(" </testsuite>\n")
		}
		b.WriteString("</testsuites>")
		fmt.Fprintln(out)
		fmt.Fprintln(out, b.String())
		fmt.Fprintln(out)
	} else {
		if outputFormat == "json" {
			finalOutput, _ = json.MarshalIndent(output, "", "  ")
		} else if outputFormat == "yaml" {
			finalOutput, _ = yaml.Marshal(output)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, string(finalOutput))
		fmt.Fprintln(out)
	}
}
