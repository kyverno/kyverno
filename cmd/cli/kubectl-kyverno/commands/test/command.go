package test

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/filter"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var cmd *cobra.Command
	var testCase string
	var fileName, gitBranch string
	var registryAccess, failOnly, removeColor, detailedResults bool
	cmd = &cobra.Command{
		Use:     "test [local folder or git repository]...",
		Args:    cobra.MinimumNArgs(1),
		Short:   command.FormatDescription(true, websiteUrl, false, description...),
		Long:    command.FormatDescription(false, websiteUrl, false, description...),
		Example: command.FormatExamples(examples...),
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
			return testCommandExecute(dirPath, fileName, gitBranch, testCase, failOnly, detailedResults)
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
) (err error) {
	// check input dir
	if len(dirPath) == 0 {
		return sanitizederror.NewWithError("a directory is required", err)
	}
	// parse filter
	filter, errors := filter.ParseFilter(testCase)
	if len(errors) > 0 {
		fmt.Println()
		fmt.Println("Filter errors:")
		for _, e := range errors {
			fmt.Println("  Error:", e)
		}
	}
	// init openapi manager
	openApiManager, err := openapi.NewManager(log.Log)
	if err != nil {
		return fmt.Errorf("unable to create open api controller, %w", err)
	}
	// load tests
	tests, err := loadTests(dirPath, fileName, gitBranch)
	if err != nil {
		fmt.Println()
		fmt.Println("Error loading tests:", err)
		return err
	}
	if len(tests) == 0 {
		fmt.Println()
		fmt.Println("No test yamls available")
	}
	if errs := tests.Errors(); len(errs) > 0 {
		fmt.Println()
		fmt.Println("Test errors:")
		for _, e := range errs {
			fmt.Println("  Path:", e.Path)
			fmt.Println("    Error:", e.Err)
		}
	}
	if len(tests) == 0 {
		if len(errors) == 0 {
			return nil
		} else {
			// TODO aggregate errors
			return errors[0]
		}
	}
	rc := &resultCounts{}
	var table table.Table
	for _, test := range tests {
		if test.Err == nil {
			// filter results
			var filteredResults []testapi.TestResults
			for _, res := range test.Test.Results {
				if filter.Apply(res) {
					filteredResults = append(filteredResults, res)
				}
			}
			if len(filteredResults) == 0 {
				continue
			}
			resourcePath := filepath.Dir(test.Path)
			responses, err := runTest(openApiManager, test, false)
			if err != nil {
				return sanitizederror.NewWithError("failed to run test", err)
			}
			t, err := printTestResult(filteredResults, responses, rc, failOnly, detailedResults, test.Fs, resourcePath)
			if err != nil {
				return sanitizederror.NewWithError("failed to print test result:", err)
			}
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
		return fmt.Errorf("%d tests failed", rc.Fail)
	}
	return nil
}

func checkResult(test testapi.TestResults, fs billy.Filesystem, resoucePath string, response engineapi.EngineResponse, rule engineapi.RuleResponse) (bool, string, string) {
	expected := test.Result
	// fallback to the deprecated field
	if expected == "" {
		expected = test.Status
	}
	// fallback on deprecated field
	if test.PatchedResource != "" {
		equals, err := getAndCompareResource(response.PatchedResource, fs, filepath.Join(resoucePath, test.PatchedResource))
		if err != nil {
			return false, err.Error(), "Resource error"
		}
		if !equals {
			return false, "Patched resource didn't match the patched resource in the test result", "Resource diff"
		}
	}
	if test.GeneratedResource != "" {
		equals, err := getAndCompareResource(rule.GeneratedResource(), fs, filepath.Join(resoucePath, test.GeneratedResource))
		if err != nil {
			return false, err.Error(), "Resource error"
		}
		if !equals {
			return false, "Generated resource didn't match the generated resource in the test result", "Resource diff"
		}
	}
	result := report.ComputePolicyReportResult(false, response, rule)
	if result.Result != expected {
		return false, result.Message, fmt.Sprintf("Want %s, got %s", expected, result.Result)
	}
	return true, result.Message, "Ok"
}

func lookupEngineResponses(test testapi.TestResults, resourceName string, responses ...engineapi.EngineResponse) []engineapi.EngineResponse {
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

func lookupRuleResponses(test testapi.TestResults, responses ...engineapi.RuleResponse) []engineapi.RuleResponse {
	var matches []engineapi.RuleResponse
	// Since there are no rules in case of validating admission policies, responses are returned without checking rule names.
	if test.IsValidatingAdmissionPolicy {
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
