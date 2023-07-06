package test

import (
	"fmt"
	"os"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/manifest"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/output/table"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Command returns version command
func Command() *cobra.Command {
	var cmd *cobra.Command
	var testCase string
	var fileName, gitBranch string
	var registryAccess, failOnly, removeColor, manifestValidate, manifestMutate, compact bool
	cmd = &cobra.Command{
		Use: "test <path_to_folder_Containing_test.yamls> [flags]\n  kyverno test <path_to_gitRepository_with_dir> --git-branch <branchName>\n  kyverno test --manifest-mutate > kyverno-test.yaml\n  kyverno test --manifest-validate > kyverno-test.yaml",
		// Args:    cobra.ExactArgs(1),
		Short:   "Run tests from directory.",
		Long:    longHelp,
		Example: exampleHelp,
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
			if manifestMutate {
				manifest.PrintMutate()
			} else if manifestValidate {
				manifest.PrintValidate()
			} else {
				store.SetRegistryAccess(registryAccess)
				_, err = testCommandExecute(dirPath, fileName, gitBranch, testCase, failOnly, false, compact)
				if err != nil {
					log.Log.V(3).Info("a directory is required")
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "kyverno-test.yaml", "test filename")
	cmd.Flags().StringVarP(&gitBranch, "git-branch", "b", "", "test github repository branch")
	cmd.Flags().StringVarP(&testCase, "test-case-selector", "t", "", `run some specific test cases by passing a string argument in double quotes to this flag like - "policy=<policy_name>, rule=<rule_name>, resource=<resource_name". The argument could be any combination of policy, rule and resource.`)
	cmd.Flags().BoolVar(&manifestMutate, "manifest-mutate", false, "prints out a template test manifest for a mutate policy")
	cmd.Flags().BoolVar(&manifestValidate, "manifest-validate", false, "prints out a template test manifest for a validate policy")
	cmd.Flags().BoolVar(&registryAccess, "registry", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().BoolVar(&failOnly, "fail-only", false, "If set to true, display all the failing test only as output for the test command")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	cmd.Flags().BoolVar(&compact, "compact", true, "Does not show detailed results")
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
	auditWarn bool,
	compact bool,
) (rc *resultCounts, err error) {
	// check input dir
	if len(dirPath) == 0 {
		return rc, sanitizederror.NewWithError("a directory is required", err)
	}
	// parse filter
	filter := parseFilter(testCase)
	// init openapi manager
	openApiManager, err := openapi.NewManager(log.Log)
	if err != nil {
		return rc, fmt.Errorf("unable to create open api controller, %w", err)
	}
	// load tests
	fs, policies, errors := loadTests(dirPath, fileName, gitBranch)
	if len(policies) == 0 {
		fmt.Printf("\n No test yamls available \n")
	}
	rc = &resultCounts{}
	var table table.Table
	for _, p := range policies {
		if reports, tests, err := applyPoliciesFromPath(
			fs,
			p.bytes,
			fs != nil,
			p.resourcePath,
			rc,
			openApiManager,
			filter,
			auditWarn,
		); err != nil {
			return rc, sanitizederror.NewWithError("failed to apply test command", err)
		} else if t, err := printTestResult(reports, tests, rc, failOnly, compact); err != nil {
			return rc, sanitizederror.NewWithError("failed to print test result:", err)
		} else {
			table.AddFailed(t.RawRows...)
		}
	}
	if len(errors) > 0 && log.Log.V(1).Enabled() {
		fmt.Println("test errors:")
		for _, e := range errors {
			fmt.Printf("    %v \n", e.Error())
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
			printFailedTestResult(table, compact)
		}
		os.Exit(1)
	}
	os.Exit(0)
	return rc, nil
}

func printTestResult(resps map[string]policyreportv1alpha2.PolicyReportResult, testResults []api.TestResults, rc *resultCounts, failOnly bool, compact bool) (table.Table, error) {
	printer := table.NewTablePrinter()
	var resultsTable table.Table
	var countDeprecatedResource int
	testCount := 1
	for _, v := range testResults {
		var row table.Row
		row.ID = testCount
		if v.Resources == nil {
			testCount++
		}
		row.Policy = color.Policy("", v.Policy)
		row.Rule = color.Rule(v.Rule)

		if v.Resources != nil {
			for _, resource := range v.Resources {
				row.ID = testCount
				testCount++
				row.Resource = color.Resource(v.Kind, v.Namespace, resource)
				var ruleNameInResultKey string
				if !v.IsVap {
					if v.AutoGeneratedRule != "" {
						ruleNameInResultKey = fmt.Sprintf("%s-%s", v.AutoGeneratedRule, v.Rule)
					} else {
						ruleNameInResultKey = v.Rule
					}
				}

				var resultKey string
				if !v.IsVap {
					resultKey = fmt.Sprintf("%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Kind, resource)
				} else {
					resultKey = fmt.Sprintf("%s-%s-%s", v.Policy, v.Kind, resource)
				}

				found, _ := isNamespacedPolicy(v.Policy)
				var ns string
				ns, v.Policy = getUserDefinedPolicyNameAndNamespace(v.Policy)
				if found && v.Namespace != "" {
					if !v.IsVap {
						resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, resource)
					} else {
						resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, v.Namespace, v.Kind, resource)
					}
				} else if found {
					if !v.IsVap {
						resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Kind, resource)
					} else {
						resultKey = fmt.Sprintf("%s-%s-%s-%s", ns, v.Policy, v.Kind, resource)
					}
					row.Policy = color.Policy(ns, v.Policy)
					row.Resource = color.Resource(v.Kind, v.Namespace, resource)
				} else if v.Namespace != "" {
					row.Resource = color.Resource(v.Kind, v.Namespace, resource)

					if !v.IsVap {
						resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, resource)
					} else {
						resultKey = fmt.Sprintf("%s-%s-%s-%s", v.Policy, v.Namespace, v.Kind, resource)
					}
				}

				var testRes policyreportv1alpha2.PolicyReportResult
				if val, ok := resps[resultKey]; ok {
					testRes = val
				} else {
					log.Log.V(2).Info("result not found", "key", resultKey)
					row.Result = color.NotFound()
					rc.Fail++
					row.IsFailure = true
					resultsTable.Add(row)
					continue
				}
				row.Message = testRes.Message
				if v.Result == "" && v.Status != "" {
					v.Result = v.Status
				}

				if testRes.Result == v.Result {
					row.Result = color.ResultPass()
					if testRes.Result == policyreportv1alpha2.StatusSkip {
						rc.Skip++
					} else {
						rc.Pass++
					}
				} else {
					log.Log.V(2).Info("result mismatch", "expected", v.Result, "received", testRes.Result, "key", resultKey)
					row.Result = color.ResultFail()
					rc.Fail++
					row.IsFailure = true
				}

				if failOnly {
					if row.Result == color.ResultFail() || row.Result == "Fail" {
						resultsTable.Add(row)
					}
				} else {
					resultsTable.Add(row)
				}
			}
		} else if v.Resource != "" {
			countDeprecatedResource++
			row.Resource = color.Resource(v.Kind, v.Namespace, v.Resource)
			var ruleNameInResultKey string
			if !v.IsVap {
				if v.AutoGeneratedRule != "" {
					ruleNameInResultKey = fmt.Sprintf("%s-%s", v.AutoGeneratedRule, v.Rule)
				} else {
					ruleNameInResultKey = v.Rule
				}
			}

			var resultKey string
			if !v.IsVap {
				resultKey = fmt.Sprintf("%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Kind, v.Resource)
			} else {
				resultKey = fmt.Sprintf("%s-%s-%s", v.Policy, v.Kind, v.Resource)
			}

			found, _ := isNamespacedPolicy(v.Policy)
			var ns string
			ns, v.Policy = getUserDefinedPolicyNameAndNamespace(v.Policy)
			if found && v.Namespace != "" {
				if !v.IsVap {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, v.Resource)
				} else {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, v.Namespace, v.Kind, v.Resource)
				}
			} else if found {
				if !v.IsVap {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Kind, v.Resource)
				} else {
					resultKey = fmt.Sprintf("%s-%s-%s-%s", ns, v.Policy, v.Kind, v.Resource)
				}

				row.Policy = color.Policy(ns, v.Policy)
				row.Resource = color.Resource(v.Kind, v.Namespace, v.Resource)
			} else if v.Namespace != "" {
				row.Resource = color.Resource(v.Kind, v.Namespace, v.Resource)

				if !v.IsVap {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, v.Resource)
				} else {
					resultKey = fmt.Sprintf("%s-%s-%s-%s", v.Policy, v.Namespace, v.Kind, v.Resource)
				}
			}

			var testRes policyreportv1alpha2.PolicyReportResult
			if val, ok := resps[resultKey]; ok {
				testRes = val
			} else {
				log.Log.V(2).Info("result not found", "key", resultKey)
				row.Result = color.NotFound()
				rc.Fail++
				row.IsFailure = true
				resultsTable.Add(row)
				continue
			}

			row.Message = testRes.Message

			if v.Result == "" && v.Status != "" {
				v.Result = v.Status
			}

			if testRes.Result == v.Result {
				row.Result = color.ResultPass()
				if testRes.Result == policyreportv1alpha2.StatusSkip {
					rc.Skip++
				} else {
					rc.Pass++
				}
			} else {
				log.Log.V(2).Info("result mismatch", "expected", v.Result, "received", testRes.Result, "key", resultKey)
				row.Result = color.ResultFail()
				rc.Fail++
				row.IsFailure = true
			}

			if failOnly {
				if row.Result == color.ResultFail() || row.Result == "Fail" {
					resultsTable.Add(row)
				}
			} else {
				resultsTable.Add(row)
			}
		}
	}
	fmt.Printf("\n")
	printer.Print(resultsTable.Rows(compact))
	return resultsTable, nil
}

func printFailedTestResult(resultsTable table.Table, compact bool) {
	printer := table.NewTablePrinter()
	for i := range resultsTable.RawRows {
		resultsTable.RawRows[i].ID = i + 1
	}
	fmt.Printf("Aggregated Failed Test Cases : ")
	fmt.Println()
	printer.Print(resultsTable.Rows(compact))
}
