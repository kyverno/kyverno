package test

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/filter"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Command() *cobra.Command {
	var testCase, outputFormat string
	var fileName, gitBranch string
	var registryAccess, failOnly, removeColor, detailedResults, requireTests bool
	cmd := &cobra.Command{
		Use:          "test [local folder or git repository]...",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, dirPath []string) (err error) {
			if len(outputFormat) > 0 {
				removeColor = true
			}
			color.Init(removeColor)
			return testCommandExecute(cmd.OutOrStdout(), dirPath, fileName, gitBranch, testCase, outputFormat, registryAccess, failOnly, detailedResults, requireTests)
		},
	}
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "kyverno-test.yaml", "Test filename")
	cmd.Flags().StringVarP(&gitBranch, "git-branch", "b", "", "Test github repository branch")
	cmd.Flags().StringVarP(&testCase, "test-case-selector", "t", "policy=*,rule=*,resource=*", "Filter test cases to run")
	cmd.Flags().StringVarP(&outputFormat, "output-format", "o", "", "Specifies the output format (json, yaml, markdown, junit)")
	cmd.Flags().BoolVar(&registryAccess, "registry", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().BoolVar(&failOnly, "fail-only", false, "If set to true, display all the failing test only as output for the test command")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	cmd.Flags().BoolVar(&detailedResults, "detailed-results", false, "If set to true, display detailed results")
	cmd.Flags().BoolVar(&requireTests, "require-tests", false, "If set to true, return an error if no tests are found")
	return cmd
}

type resultCounts struct {
	Skip int
	Pass int
	Fail int
}

func testCommandExecute(
	out io.Writer,
	dirPath []string,
	fileName string,
	gitBranch string,
	testCase string,
	outputFormat string,
	registryAccess bool,
	failOnly bool,
	detailedResults bool,
	requireTests bool,
) (err error) {
	// check input dir
	if len(dirPath) == 0 {
		return fmt.Errorf("a directory is required")
	}
	// check correct format output
	if len(outputFormat) > 0 {
		validFormats := map[string]bool{
			"json":     true,
			"yaml":     true,
			"markdown": true,
			"junit":    true,
		}
		if !validFormats[outputFormat] {
			return fmt.Errorf("invalid format, expected (json, yaml, markdown, junit)")
		}
	}
	// fetch resource filters
	resourceFilters := filter.ExtractResourceFilters(testCase)
	// parse filter
	filter, errors := filter.ParseFilter(testCase)
	if len(errors) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Filter errors:")
		for _, e := range errors {
			fmt.Fprintln(out, "  Error:", e)
		}
	}
	// load tests
	tests, err := loadTests(dirPath, fileName, gitBranch)
	if err != nil {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Error loading tests:", err)
		return err
	}
	if len(tests) == 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "No test yamls available")
	}
	if errs := tests.Errors(); len(errs) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Test errors:")
		for _, e := range errs {
			fmt.Fprintln(out, "  Path:", e.Path)
			fmt.Fprintln(out, "    Error:", e.Err)
		}
		return fmt.Errorf("found %d errors after loading tests", len(errs))
	}
	if len(tests) == 0 {
		if requireTests {
			return fmt.Errorf("no tests found")
		}

		if len(errors) == 0 {
			return nil
		} else {
			// TODO aggregate errors
			return errors[0]
		}
	}
	rc := &resultCounts{}
	var fullTable table.Table
	for _, test := range tests {
		if test.Err == nil {
			if deprecations.CheckTest(out, test.Path, test.Test) {
				return fmt.Errorf("test file %s uses a deprecated schema — please migrate to the latest format", test.Path)
			}

			// filter results
			var filteredResults []v1alpha1.TestResult
			for _, res := range test.Test.Results {
				if filter.Apply(res) {
					if len(resourceFilters) > 0 {
						res.Resources = resourceFilters
					}
					filteredResults = append(filteredResults, res)
				}
			}
			if len(filteredResults) == 0 {
				continue
			}
			resourcePath := filepath.Dir(test.Path)
			responses, err := runTest(out, test, registryAccess)
			if err != nil {
				return fmt.Errorf("failed to run test (%w)", err)
			}
			fmt.Fprintln(out, "  Checking results ...")
			var resultsTable table.Table
			if err := printTestResult(filteredResults, responses, rc, &resultsTable, test.Fs, resourcePath); err != nil {
				return fmt.Errorf("failed to print test result (%w)", err)
			}
			if err := printCheckResult(test.Test.Checks, *responses, rc, &resultsTable); err != nil {
				return fmt.Errorf("failed to print test result (%w)", err)
			}
			fullTable.AddFailed(resultsTable.RawRows...)
			if !failOnly {
				if len(outputFormat) > 0 {
					printOutputFormats(out, outputFormat, resultsTable, detailedResults)
				} else {
					printer := table.NewTablePrinter(out)
					fmt.Fprintln(out)
					printer.Print(resultsTable.Rows(detailedResults))
					fmt.Fprintln(out)
				}
			}
		}
	}
	if !failOnly {
		fmt.Fprintf(out, "\nTest Summary: %d tests passed and %d tests failed\n", rc.Pass+rc.Skip, rc.Fail)
	} else {
		fmt.Fprintf(out, "\nTest Summary: %d out of %d tests failed\n", rc.Fail, rc.Pass+rc.Skip+rc.Fail)
	}
	fmt.Fprintln(out)
	if rc.Fail > 0 {
		if failOnly {
			if len(outputFormat) > 0 {
				printOutputFormats(out, outputFormat, fullTable, detailedResults)
			} else {
				printFailedTestResult(out, fullTable, detailedResults)
			}
		}
		return fmt.Errorf("%d tests failed", rc.Fail)
	}
	return nil
}

func checkResult(test v1alpha1.TestResult, fs billy.Filesystem, resoucePath string, response engineapi.EngineResponse, rule engineapi.RuleResponse, actualResource unstructured.Unstructured) (bool, string, string) {
	expected := test.Result
	expectedPatchResources := test.PatchedResources
	if expectedPatchResources != "" {
		equals, diff, err := getAndCompareResource(actualResource, fs, filepath.Join(resoucePath, expectedPatchResources))
		if err != nil {
			return false, err.Error(), "Resource error"
		}
		if !equals {
			dmp := diffmatchpatch.New()
			legend := dmp.DiffPrettyText(dmp.DiffMain("only in expected", "only in actual", false))
			return false, fmt.Sprintf("Patched resource didn't match the patched resource in the test result\n(%s)\n\n%s", legend, diff), "Resource diff"
		}
	}
	if test.GeneratedResource != "" {
		equals, diff, err := getAndCompareResource(actualResource, fs, filepath.Join(resoucePath, test.GeneratedResource))
		if err != nil {
			return false, err.Error(), "Resource error"
		}
		if !equals {
			dmp := diffmatchpatch.New()
			legend := dmp.DiffPrettyText(dmp.DiffMain("only in expected", "only in actual", false))
			return false, fmt.Sprintf("Patched resource didn't match the generated resource in the test result\n(%s)\n\n%s", legend, diff), "Resource diff"
		}
	}
	result := report.ComputePolicyReportResult(false, response, rule)
	if result.Result != expected {
		return false, result.Description, fmt.Sprintf("Want %s, got %s", expected, result.Result)
	}
	return true, result.Description, "Ok"
}

func lookupRuleResponses(test v1alpha1.TestResult, responses ...engineapi.RuleResponse) []engineapi.RuleResponse {
	var matches []engineapi.RuleResponse
	// Since there are no rules in case of validating admission policies, responses are returned without checking rule names.
	if test.IsValidatingAdmissionPolicy || test.IsValidatingPolicy || test.IsImageValidatingPolicy || test.IsMutatingAdmissionPolicy || test.IsDeletingPolicy || test.IsGeneratingPolicy || test.IsMutatingPolicy {
		matches = responses
	} else {
		for _, response := range responses {
			rule := response.Name()
			if rule != test.Rule && rule != "autogen-"+test.Rule && rule != "autogen-cronjob-"+test.Rule {
				continue
			}
			matches = append(matches, response)
		}
	}
	return matches
}
