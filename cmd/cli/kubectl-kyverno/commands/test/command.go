package test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test/api"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/color"
	filterutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/filter"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/output/table"
	reportutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/report"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Command() *cobra.Command {
	var cmd *cobra.Command
	var testCase string
	var fileName, gitBranch string
	var registryAccess, failOnly, removeColor, detailedResults bool
	cmd = &cobra.Command{
		Use:     "test [local folder or git repository]...",
		Args:    cobra.MinimumNArgs(1),
		Short:   cobrautils.FormatDescription(true, websiteUrl, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, dirPath []string) (err error) {
			color.InitColors(removeColor)
			defer func() {
				if err != nil {
					if !sanitizederror.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")
					}
				}
			}()
			store.SetRegistryAccess(registryAccess)
			_, err = testCommandExecute(dirPath, fileName, gitBranch, testCase, failOnly, detailedResults)
			if err != nil {
				log.Log.V(3).Info("a directory is required")
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "kyverno-test.yaml", "Test filename")
	cmd.Flags().StringVarP(&gitBranch, "git-branch", "b", "", "Test github repository branch")
	cmd.Flags().StringVarP(&testCase, "test-case-selector", "t", "policy=*,rule=*,resource=*", "Filter test cases to run")
	cmd.Flags().BoolVar(&registryAccess, "registry", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().BoolVar(&failOnly, "fail-only", false, "If set to true, display all the failing test only as output for the test command")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	cmd.Flags().BoolVar(&detailedResults, "detailed-results", false, "If set to true, display detailed results")
	return cmd
}

type resultCounts struct {
	Skip int
	Pass int
	Fail int
}

func testCommandExecute(
	dirPath []string,
	fileName string,
	gitBranch string,
	testCase string,
	failOnly bool,
	detailedResults bool,
) (rc *resultCounts, err error) {
	// check input dir
	if len(dirPath) == 0 {
		return rc, sanitizederror.NewWithError("a directory is required", err)
	}
	// parse filter
	filter, errors := filterutils.ParseFilter(testCase)
	if len(errors) > 0 {
		fmt.Println()
		fmt.Println("Filter errors:")
		for _, e := range errors {
			fmt.Printf("    %v \n", e.Error())
		}
	}
	// init openapi manager
	openApiManager, err := openapi.NewManager(log.Log)
	if err != nil {
		return rc, fmt.Errorf("unable to create open api controller, %w", err)
	}
	// load tests
	fs, tests, err := loadTests(dirPath, fileName, gitBranch)
	if err != nil {
		fmt.Println()
		fmt.Println("Error loading tests:")
		fmt.Printf("    %s\n", err)
		return rc, err
	}
	if len(tests) == 0 {
		fmt.Println()
		fmt.Println("No test yamls available")
	}
	if errs := tests.Errors(); len(errs) > 0 {
		fmt.Println()
		fmt.Println("Test errors:")
		for _, e := range errs {
			fmt.Printf("    %v \n", e.Error())
		}
	}
	if len(tests) == 0 {
		if len(errors) == 0 {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	rc = &resultCounts{}
	var table table.Table
	for _, p := range tests {
		resourcePath := filepath.Dir(p.Path)
		if tests, responses, err := applyPoliciesFromPath(
			fs,
			p.Test,
			fs != nil,
			resourcePath,
			rc,
			openApiManager,
			filter,
			false,
		); err != nil {
			return rc, sanitizederror.NewWithError("failed to apply test command", err)
		} else if t, err := printTestResult(tests, responses, rc, failOnly, detailedResults, fs, resourcePath); err != nil {
			return rc, sanitizederror.NewWithError("failed to print test result:", err)
		} else {
			table.AddFailed(t.RawRows...)
		}
	}
	if !failOnly {
		fmt.Printf("\nTest Summary: %d tests passed and %d tests failed\n", rc.Pass+rc.Skip, rc.Fail)
	} else {
		fmt.Printf("\nTest Summary: %d out of %d tests failed\n", rc.Fail, rc.Pass+rc.Skip+rc.Fail)
	}
	fmt.Println()
	if rc.Fail > 0 {
		if !failOnly {
			printFailedTestResult(table, detailedResults)
		}
		os.Exit(1)
	}
	os.Exit(0)
	return rc, nil
}

func checkResult(test api.TestResults, fs billy.Filesystem, resoucePath string, response engineapi.EngineResponse, rule engineapi.RuleResponse) (bool, string, string) {
	expected := test.Result
	// fallback to the deprecated field
	if expected == "" {
		expected = test.Status
	}
	// fallback on deprecated field
	if test.PatchedResource != "" {
		equals, err := getAndCompareResource(test.PatchedResource, response.PatchedResource, fs, resoucePath, false)
		if err != nil {
			return false, err.Error(), "Resource error"
		}
		if !equals {
			return false, "Patched resource didn't match the patched resource in the test result", "Resource diff"
		}
	}
	if test.GeneratedResource != "" {
		equals, err := getAndCompareResource(test.GeneratedResource, rule.GeneratedResource(), fs, resoucePath, true)
		if err != nil {
			return false, err.Error(), "Resource error"
		}
		if !equals {
			return false, "Generated resource didn't match the generated resource in the test result", "Resource diff"
		}
	}
	result := reportutils.ComputePolicyReportResult(false, response, rule)
	if result.Result != expected {
		return false, result.Message, fmt.Sprintf("Want %s, got %s", expected, result.Result)
	}
	return true, result.Message, "Ok"
}

func lookupEngineResponses(test api.TestResults, resourceName string, responses ...engineapi.EngineResponse) []engineapi.EngineResponse {
	var matches []engineapi.EngineResponse
	for _, response := range responses {
		policy := response.Policy()
		resource := response.Resource
		if policy.GetName() != test.Policy {
			continue
		}
		if test.Kind != resource.GetKind() {
			continue
		}
		if resourceName != "" && resourceName != resource.GetName() {
			continue
		}
		if test.Namespace != "" && test.Namespace != resource.GetNamespace() {
			continue
		}
		matches = append(matches, response)
	}
	return matches
}

func lookupRuleResponses(test api.TestResults, responses ...engineapi.RuleResponse) []engineapi.RuleResponse {
	var matches []engineapi.RuleResponse
	for _, response := range responses {
		rule := response.Name()
		if rule != test.Rule && rule != "autogen-"+test.Rule && rule != "autogen-cronjob-"+test.Rule {
			continue
		}
		matches = append(matches, response)
	}
	return matches
}

func printTestResult(
	tests []api.TestResults,
	responses []engineapi.EngineResponse,
	rc *resultCounts,
	failOnly bool,
	detailedResults bool,
	fs billy.Filesystem,
	resoucePath string,
) (table.Table, error) {
	printer := table.NewTablePrinter()
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
						CompactRow: table.CompactRow{
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
					CompactRow: table.CompactRow{
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
	fmt.Printf("\n")
	printer.Print(resultsTable.Rows(detailedResults))
	return resultsTable, nil
}

func printFailedTestResult(resultsTable table.Table, detailedResults bool) {
	printer := table.NewTablePrinter()
	for i := range resultsTable.RawRows {
		resultsTable.RawRows[i].ID = i + 1
	}
	fmt.Printf("Aggregated Failed Test Cases : ")
	fmt.Println()
	printer.Print(resultsTable.Rows(detailedResults))
}
