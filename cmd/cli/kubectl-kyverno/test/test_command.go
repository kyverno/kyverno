package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/manifest"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var longHelp = `
The test command provides a facility to test resources against policies by comparing expected results, declared ahead of time in a test manifest file, to actual results reported by Kyverno. Users provide the path to the folder containing a kyverno-test.yaml file where the location could be on a local filesystem or a remote git repository.
`

var exampleHelp = `
# Test a git repository containing Kyverno test cases.
kyverno test https://github.com/kyverno/policies/pod-security --git-branch main
<snip>

Executing require-non-root-groups...
applying 1 policy to 2 resources...

│───│─────────────────────────│──────────────────────────│──────────────────────────────────│────────│
│ # │ POLICY                  │ RULE                     │ RESOURCE                         │ RESULT │
│───│─────────────────────────│──────────────────────────│──────────────────────────────────│────────│
│ 1 │ require-non-root-groups │ check-runasgroup         │ default/Pod/fs-group0            │ Pass   │
│ 2 │ require-non-root-groups │ check-supplementalGroups │ default/Pod/fs-group0            │ Pass   │
│ 3 │ require-non-root-groups │ check-fsGroup            │ default/Pod/fs-group0            │ Pass   │
│ 4 │ require-non-root-groups │ check-supplementalGroups │ default/Pod/supplemental-groups0 │ Pass   │
│ 5 │ require-non-root-groups │ check-fsGroup            │ default/Pod/supplemental-groups0 │ Pass   │
│ 6 │ require-non-root-groups │ check-runasgroup         │ default/Pod/supplemental-groups0 │ Pass   │
│───│─────────────────────────│──────────────────────────│──────────────────────────────────│────────│
<snip>

# Test a local folder containing test cases.
kyverno test .

Executing limit-containers-per-pod...
applying 1 policy to 4 resources...

│───│──────────────────────────│──────────────────────────────────────│─────────────────────────────│────────│
│ # │ POLICY                   │ RULE                                 │ RESOURCE                    │ RESULT │
│───│──────────────────────────│──────────────────────────────────────│─────────────────────────────│────────│
│ 1 │ limit-containers-per-pod │ limit-containers-per-pod-bare        │ default/Pod/myapp-pod-1     │ Pass   │
│ 2 │ limit-containers-per-pod │ limit-containers-per-pod-bare        │ default/Pod/myapp-pod-2     │ Pass   │
│ 3 │ limit-containers-per-pod │ limit-containers-per-pod-controllers │ default/Deployment/mydeploy │ Pass   │
│ 4 │ limit-containers-per-pod │ limit-containers-per-pod-cronjob     │ default/CronJob/mycronjob   │ Pass   │
│───│──────────────────────────│──────────────────────────────────────│─────────────────────────────│────────│

Test Summary: 4 tests passed and 0 tests failed

# Test some specific test cases out of many test cases in a local folder.
kyverno test . --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"

Executing test-simple...
applying 1 policy to 1 resource...

│───│─────────────────────│───────────────────│─────────────────────────────────────────│────────│
│ # │ POLICY              │ RULE              │ RESOURCE                                │ RESULT │
│───│─────────────────────│───────────────────│─────────────────────────────────────────│────────│
│ 1 │ disallow-latest-tag │ require-image-tag │ default/Pod/test-require-image-tag-pass │ Pass   │
│───│─────────────────────│───────────────────│─────────────────────────────────────────│────────│

Test Summary: 1 tests passed and 0 tests failed



**TEST FILE STRUCTURE**:

The kyverno-test.yaml has four parts:
	"policies"   --> List of policies which are applied.
	"resources"  --> List of resources on which the policies are applied.
	"variables"  --> Variable file path containing variables referenced in the policy (OPTIONAL).
	"results"    --> List of results expected after applying the policies to the resources.

** TEST FILE FORMAT**:

name: <test_name>
policies:
- <path/to/policy1.yaml>
- <path/to/policy2.yaml>
resources:
- <path/to/resource1.yaml>
- <path/to/resource2.yaml>
variables: <variable_file> (OPTIONAL)
results:
- policy: <name> (For Namespaced [Policy] files, format is <policy_namespace>/<policy_name>)
  rule: <name>
  resource: <name>
  namespace: <name> (OPTIONAL)
  kind: <name>
  patchedResource: <path/to/patched/resource.yaml> (For mutate policies/rules only)
  result: <pass|fail|skip>

**VARIABLES FILE FORMAT**:

policies:
- name: <policy_name>
  rules:
  - name: <rule_name>
    # Global variable values
    values:
      foo: bar
  resources:
  - name: <resource_name_1>
    # Resource-specific variable values
    values:
      foo: baz
  - name: <resource_name_2>
    values:
      foo: bin
# If policy is matching on Kind/Subresource, then this is required
subresources:
  - subresource:
      name: <name of subresource>
      kind: <kind of subresource>
      group: <group of subresource>
      version: <version of subresource>
    parentResource:
      name: <name of parent resource>
      kind: <kind of parent resource>
      group: <group of parent resource>
      version: <version of parent resource>

**RESULT DESCRIPTIONS**:

pass  --> The resource is either validated by the policy or, if a mutation, equals the state of the patched resource.
fail  --> The resource fails validation or the patched resource generated by Kyverno is not equal to the input resource provided by the user.
skip  --> The rule is not applied.

For more information visit https://kyverno.io/docs/kyverno-cli/#test
`

// Command returns version command
func Command() *cobra.Command {
	var cmd *cobra.Command
	var testCase string
	var fileName, gitBranch string
	var registryAccess, failOnly, removeColor, manifestValidate, manifestMutate bool
	cmd = &cobra.Command{
		Use: "test <path_to_folder_Containing_test.yamls> [flags]\n  kyverno test <path_to_gitRepository_with_dir> --git-branch <branchName>\n  kyverno test --manifest-mutate > kyverno-test.yaml\n  kyverno test --manifest-validate > kyverno-test.yaml",
		// Args:    cobra.ExactArgs(1),
		Short:   "Run tests from directory.",
		Long:    longHelp,
		Example: exampleHelp,
		RunE: func(cmd *cobra.Command, dirPath []string) (err error) {
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
				_, err = testCommandExecute(dirPath, fileName, gitBranch, testCase, failOnly, removeColor, false)
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
	cmd.Flags().BoolVarP(&manifestMutate, "manifest-mutate", "", false, "prints out a template test manifest for a mutate policy")
	cmd.Flags().BoolVarP(&manifestValidate, "manifest-validate", "", false, "prints out a template test manifest for a validate policy")
	cmd.Flags().BoolVarP(&registryAccess, "registry", "", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().BoolVarP(&failOnly, "fail-only", "", false, "If set to true, display all the failing test only as output for the test command")
	cmd.Flags().BoolVarP(&removeColor, "remove-color", "", false, "Remove any color from output")
	return cmd
}

type Table struct {
	ID       int    `header:"#"`
	Policy   string `header:"policy"`
	Rule     string `header:"rule"`
	Resource string `header:"resource"`
	Result   string `header:"result"`
}

type resultCounts struct {
	Skip int
	Pass int
	Fail int
}

type testFilter struct {
	policy   string
	rule     string
	resource string
	enabled  bool
}

var ftable []Table

func testCommandExecute(
	dirPath []string,
	fileName string,
	gitBranch string,
	testCase string,
	failOnly bool,
	removeColor bool,
	auditWarn bool,
) (rc *resultCounts, err error) {
	var errors []error
	fs := memfs.New()
	rc = &resultCounts{}
	var testYamlCount int
	tf := &testFilter{
		enabled: true,
	}

	if len(dirPath) == 0 {
		return rc, sanitizederror.NewWithError("a directory is required", err)
	}

	if len(testCase) != 0 {
		parameters := map[string]string{"policy": "", "rule": "", "resource": ""}

		for _, t := range strings.Split(testCase, ",") {
			if !strings.Contains(t, "=") {
				fmt.Printf("\n Invalid test-case-selector argument. Selecting all test cases. \n")
				tf.enabled = false
				break
			}

			key := strings.TrimSpace(strings.Split(t, "=")[0])
			value := strings.TrimSpace(strings.Split(t, "=")[1])

			_, ok := parameters[key]
			if !ok {
				fmt.Printf("\n Invalid parameter. Parameter can only be policy, rule or resource. Selecting all test cases \n")
				tf.enabled = false
				break
			}

			parameters[key] = value
		}

		tf.policy = parameters["policy"]
		tf.rule = parameters["rule"]
		tf.resource = parameters["resource"]
	} else {
		tf.enabled = false
	}

	openApiManager, err := openapi.NewManager(log.Log)
	if err != nil {
		return rc, fmt.Errorf("unable to create open api controller, %w", err)
	}
	if strings.Contains(dirPath[0], "https://") {
		gitURL, err := url.Parse(dirPath[0])
		if err != nil {
			return rc, sanitizederror.NewWithError("failed to parse URL", err)
		}

		pathElems := strings.Split(gitURL.Path[1:], "/")
		if len(pathElems) <= 1 {
			err := fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch (without --git-branch flag) OR https://github.com/:owner/:repository/:directory (with --git-branch flag)", gitURL.Path)
			fmt.Printf("Error: failed to parse URL \nCause: %s\n", err)
			os.Exit(1)
		}

		gitURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
		repoURL := gitURL.String()

		var gitPathToYamls string
		if gitBranch == "" {
			gitPathToYamls = "/"

			if string(dirPath[0][len(dirPath[0])-1]) == "/" {
				gitBranch = strings.ReplaceAll(dirPath[0], repoURL+"/", "")
			} else {
				gitBranch = strings.ReplaceAll(dirPath[0], repoURL, "")
			}

			if gitBranch == "" {
				gitBranch = "main"
			} else if string(gitBranch[0]) == "/" {
				gitBranch = gitBranch[1:]
			}
		} else {
			if string(dirPath[0][len(dirPath[0])-1]) == "/" {
				gitPathToYamls = strings.ReplaceAll(dirPath[0], repoURL+"/", "/")
			} else {
				gitPathToYamls = strings.ReplaceAll(dirPath[0], repoURL, "/")
			}
		}

		_, cloneErr := gitutils.Clone(repoURL, fs, gitBranch)
		if cloneErr != nil {
			fmt.Printf("Error: failed to clone repository \nCause: %s\n", cloneErr)
			log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", cloneErr)
			os.Exit(1)
		}

		policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls)
		if err != nil {
			return rc, sanitizederror.NewWithError("failed to list YAMLs in repository", err)
		}
		sort.Strings(policyYamls)

		for _, yamlFilePath := range policyYamls {
			file, err := fs.Open(yamlFilePath)
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("Error: failed to open file", err))
				continue
			}

			if path.Base(file.Name()) == fileName {
				testYamlCount++
				policyresoucePath := strings.Trim(yamlFilePath, fileName)
				bytes, err := io.ReadAll(file)
				if err != nil {
					errors = append(errors, sanitizederror.NewWithError("Error: failed to read file", err))
					continue
				}

				policyBytes, err := yaml.ToJSON(bytes)
				if err != nil {
					errors = append(errors, sanitizederror.NewWithError("failed to convert to JSON", err))
					continue
				}
				if err := applyPoliciesFromPath(fs, policyBytes, true, policyresoucePath, rc, openApiManager, tf, failOnly, removeColor, auditWarn); err != nil {
					return rc, sanitizederror.NewWithError("failed to apply test command", err)
				}
			}
		}

		if testYamlCount == 0 {
			fmt.Printf("\n No test yamls available \n")
		}
	} else {
		var testFiles int
		path := filepath.Clean(dirPath[0])
		errors = getLocalDirTestFiles(fs, path, fileName, rc, &testFiles, openApiManager, tf, failOnly, removeColor, auditWarn)

		if testFiles == 0 {
			fmt.Printf("\n No test files found. Please provide test YAML files named kyverno-test.yaml \n")
		}
	}

	if len(errors) > 0 && log.Log.V(1).Enabled() {
		fmt.Printf("test errors: \n")
		for _, e := range errors {
			fmt.Printf("    %v \n", e.Error())
		}
	}

	if !failOnly {
		fmt.Printf("\nTest Summary: %d tests passed and %d tests failed\n", rc.Pass+rc.Skip, rc.Fail)
	} else {
		fmt.Printf("\nTest Summary: %d out of %d tests failed\n", rc.Fail, rc.Pass+rc.Skip+rc.Fail)
	}
	fmt.Printf("\n")

	if rc.Fail > 0 && !failOnly {
		printFailedTestResult(removeColor)
		os.Exit(1)
	}
	os.Exit(0)
	return rc, nil
}

func getLocalDirTestFiles(
	fs billy.Filesystem,
	path string,
	fileName string,
	rc *resultCounts,
	testFiles *int,
	openApiManager openapi.Manager,
	tf *testFilter,
	failOnly bool,
	removeColor bool,
	auditWarn bool,
) []error {
	var errors []error

	files, err := os.ReadDir(path)
	if err != nil {
		return []error{fmt.Errorf("failed to read %v: %v", path, err.Error())}
	}
	for _, file := range files {
		if file.IsDir() {
			getLocalDirTestFiles(fs, filepath.Join(path, file.Name()), fileName, rc, testFiles, openApiManager, tf, failOnly, removeColor, auditWarn)
			continue
		}
		if file.Name() == fileName {
			*testFiles++
			// We accept the risk of including files here as we read the test dir only.
			yamlFile, err := os.ReadFile(filepath.Join(path, file.Name())) // #nosec G304
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("unable to read yaml", err))
				continue
			}
			valuesBytes, err := yaml.ToJSON(yamlFile)
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("failed to convert json", err))
				continue
			}
			if err := applyPoliciesFromPath(fs, valuesBytes, false, path, rc, openApiManager, tf, failOnly, removeColor, auditWarn); err != nil {
				errors = append(errors, sanitizederror.NewWithError(fmt.Sprintf("failed to apply test command from file %s", file.Name()), err))
				continue
			}
		}
	}
	return errors
}

func buildPolicyResults(
	engineResponses []engineapi.EngineResponse,
	testResults []api.TestResults,
	policyResourcePath string,
	fs billy.Filesystem,
	isGit bool,
	auditWarn bool,
) (map[string]policyreportv1alpha2.PolicyReportResult, []api.TestResults) {
	results := map[string]policyreportv1alpha2.PolicyReportResult{}

	for _, resp := range engineResponses {
		policyName := resp.Policy().GetName()
		resourceName := resp.Resource.GetName()
		resourceKind := resp.Resource.GetKind()
		resourceNamespace := resp.Resource.GetNamespace()
		policyNamespace := resp.Policy().GetNamespace()

		var rules []string
		for _, rule := range resp.PolicyResponse.Rules {
			rules = append(rules, rule.Name())
		}

		result := policyreportv1alpha2.PolicyReportResult{
			Policy: policyName,
			Resources: []corev1.ObjectReference{
				{
					Name: resourceName,
				},
			},
			Message: buildMessage(resp),
		}

		var patchedResourcePath []string
		for i, test := range testResults {
			var userDefinedPolicyNamespace string
			var userDefinedPolicyName string
			found, err := isNamespacedPolicy(test.Policy)
			if err != nil {
				log.Log.V(3).Info("error while checking the policy is namespaced or not", "policy: ", test.Policy, "error: ", err)
				continue
			}

			if found {
				userDefinedPolicyNamespace, userDefinedPolicyName = getUserDefinedPolicyNameAndNamespace(test.Policy)
				test.Policy = userDefinedPolicyName
			}

			if test.Resources != nil {
				if test.Policy == policyName {
					// results[].namespace value implicit set same as metadata.namespace until and unless
					// user provides explicit values for results[].namespace in test yaml file.
					if test.Namespace == "" {
						test.Namespace = resourceNamespace
						testResults[i].Namespace = resourceNamespace
					}
					for _, resource := range test.Resources {
						if resource == resourceName {
							var resultsKey string
							resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, resource)
							if !slices.Contains(rules, test.Rule) {
								if !slices.Contains(rules, "autogen-"+test.Rule) {
									if !slices.Contains(rules, "autogen-cronjob-"+test.Rule) {
										result.Result = policyreportv1alpha2.StatusSkip
									} else {
										testResults[i].AutoGeneratedRule = "autogen-cronjob"
										test.Rule = "autogen-cronjob-" + test.Rule
										resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, resource)
									}
								} else {
									testResults[i].AutoGeneratedRule = "autogen"
									test.Rule = "autogen-" + test.Rule
									resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, resource)
								}

								if results[resultsKey].Result == "" {
									result.Result = policyreportv1alpha2.StatusSkip
									results[resultsKey] = result
								}
							}

							patchedResourcePath = append(patchedResourcePath, test.PatchedResource)
							if _, ok := results[resultsKey]; !ok {
								results[resultsKey] = result
							}
						}
					}
				}
			}
			if test.Resource != "" {
				if test.Policy == policyName && test.Resource == resourceName {
					var resultsKey string
					resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource)
					if !slices.Contains(rules, test.Rule) {
						if !slices.Contains(rules, "autogen-"+test.Rule) {
							if !slices.Contains(rules, "autogen-cronjob-"+test.Rule) {
								result.Result = policyreportv1alpha2.StatusSkip
							} else {
								testResults[i].AutoGeneratedRule = "autogen-cronjob"
								test.Rule = "autogen-cronjob-" + test.Rule
								resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource)
							}
						} else {
							testResults[i].AutoGeneratedRule = "autogen"
							test.Rule = "autogen-" + test.Rule
							resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource)
						}

						if results[resultsKey].Result == "" {
							result.Result = policyreportv1alpha2.StatusSkip
							results[resultsKey] = result
						}
					}

					patchedResourcePath = append(patchedResourcePath, test.PatchedResource)
					if _, ok := results[resultsKey]; !ok {
						results[resultsKey] = result
					}
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.RuleType() != engineapi.Generation || test.Rule != rule.Name() {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name(), resourceNamespace, resourceKind, resourceName)
				for _, key := range resultsKey {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}

					if rule.Status() == engineapi.RuleStatusSkip {
						result.Result = policyreportv1alpha2.StatusSkip
					} else if rule.Status() == engineapi.RuleStatusError {
						result.Result = policyreportv1alpha2.StatusError
					} else {
						var x string
						result.Result = policyreportv1alpha2.StatusFail
						x = getAndCompareResource(test.GeneratedResource, rule.GeneratedResource(), isGit, policyResourcePath, fs, true)
						if x == "pass" {
							result.Result = policyreportv1alpha2.StatusPass
						}
					}
					results[resultKey] = result
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.RuleType() != engineapi.Mutation || test.Rule != rule.Name() {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name(), resourceNamespace, resourceKind, resourceName)
				for _, key := range resultsKey {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}

					if rule.Status() == engineapi.RuleStatusSkip {
						result.Result = policyreportv1alpha2.StatusSkip
					} else if rule.Status() == engineapi.RuleStatusError {
						result.Result = policyreportv1alpha2.StatusError
					} else {
						var x string
						for _, path := range patchedResourcePath {
							result.Result = policyreportv1alpha2.StatusFail
							x = getAndCompareResource(path, resp.PatchedResource, isGit, policyResourcePath, fs, false)
							if x == "pass" {
								result.Result = policyreportv1alpha2.StatusPass
								break
							}
						}
					}

					results[resultKey] = result
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.RuleType() != engineapi.Validation && rule.RuleType() != engineapi.ImageVerify || test.Rule != rule.Name() {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name(), resourceNamespace, resourceKind, resourceName)
				for _, key := range resultsKey {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}

					ann := resp.Policy().GetAnnotations()
					if rule.Status() == engineapi.RuleStatusSkip {
						result.Result = policyreportv1alpha2.StatusSkip
					} else if rule.Status() == engineapi.RuleStatusError {
						result.Result = policyreportv1alpha2.StatusError
					} else if rule.Status() == engineapi.RuleStatusPass {
						result.Result = policyreportv1alpha2.StatusPass
					} else if rule.Status() == engineapi.RuleStatusFail {
						if scored, ok := ann[kyvernov1.AnnotationPolicyScored]; ok && scored == "false" {
							result.Result = policyreportv1alpha2.StatusWarn
						} else if auditWarn && resp.GetValidationFailureAction().Audit() {
							result.Result = policyreportv1alpha2.StatusWarn
						} else {
							result.Result = policyreportv1alpha2.StatusFail
						}
					} else {
						fmt.Println(rule)
					}

					results[resultKey] = result
				}
			}
		}
	}
	return results, testResults
}

func GetAllPossibleResultsKey(policyNamespace, policy, rule, resourceNamespace, kind, resource string) []string {
	var resultsKey []string
	resultKey1 := fmt.Sprintf("%s-%s-%s-%s", policy, rule, kind, resource)
	resultKey2 := fmt.Sprintf("%s-%s-%s-%s-%s", policy, rule, resourceNamespace, kind, resource)
	resultKey3 := fmt.Sprintf("%s-%s-%s-%s-%s", policyNamespace, policy, rule, kind, resource)
	resultKey4 := fmt.Sprintf("%s-%s-%s-%s-%s-%s", policyNamespace, policy, rule, resourceNamespace, kind, resource)
	resultsKey = append(resultsKey, resultKey1, resultKey2, resultKey3, resultKey4)
	return resultsKey
}

func GetResultKeyAccordingToTestResults(policyNs, policy, rule, resourceNs, kind, resource string) string {
	var resultKey string
	resultKey = fmt.Sprintf("%s-%s-%s-%s", policy, rule, kind, resource)

	if policyNs != "" && resourceNs != "" {
		resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", policyNs, policy, rule, resourceNs, kind, resource)
	} else if policyNs != "" {
		resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policyNs, policy, rule, kind, resource)
	} else if resourceNs != "" {
		resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policy, rule, resourceNs, kind, resource)
	}
	return resultKey
}

func isNamespacedPolicy(policyNames string) (bool, error) {
	return regexp.MatchString("^[a-z]*/[a-z]*", policyNames)
}

func getUserDefinedPolicyNameAndNamespace(policyName string) (string, string) {
	if strings.Contains(policyName, "/") {
		parts := strings.Split(policyName, "/")
		namespace := parts[0]
		policy := parts[1]
		return namespace, policy
	}
	return "", policyName
}

// getAndCompareResource --> Get the patchedResource or generatedResource from the path provided by user
// And compare this resource with engine generated resource.
func getAndCompareResource(path string, engineResource unstructured.Unstructured, isGit bool, policyResourcePath string, fs billy.Filesystem, isGenerate bool) string {
	var status string
	resourceType := "patchedResource"
	if isGenerate {
		resourceType = "generatedResource"
	}

	userResource, err := common.GetResourceFromPath(fs, path, isGit, policyResourcePath, resourceType)
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		return ""
	}
	matched, err := generate.ValidateResourceWithPattern(log.Log, engineResource.UnstructuredContent(), userResource.UnstructuredContent())
	if err != nil {
		log.Log.V(3).Info(resourceType+" mismatch", "error", err.Error())
		status = "fail"
	} else if matched == "" {
		status = "pass"
	}
	return status
}

func buildMessage(resp engineapi.EngineResponse) string {
	var bldr strings.Builder
	for _, ruleResp := range resp.PolicyResponse.Rules {
		fmt.Fprintf(&bldr, "  %s: %s \n", ruleResp.Name(), ruleResp.Status())
		fmt.Fprintf(&bldr, "    %s \n", ruleResp.Message())
	}

	return bldr.String()
}

func getFullPath(paths []string, policyResourcePath string, isGit bool) []string {
	var pols []string
	var pol string
	if !isGit {
		for _, path := range paths {
			pol = filepath.Join(policyResourcePath, path)
			pols = append(pols, pol)
		}
		return pols
	}
	return paths
}

func applyPoliciesFromPath(
	fs billy.Filesystem,
	policyBytes []byte,
	isGit bool,
	policyResourcePath string,
	rc *resultCounts,
	openApiManager openapi.Manager,
	tf *testFilter,
	failOnly bool,
	removeColor bool,
	auditWarn bool,
) (err error) {
	engineResponses := make([]engineapi.EngineResponse, 0)
	var dClient dclient.Interface
	values := &api.Test{}
	var variablesString string
	var resultCounts common.ResultCounts

	store.SetLocal(true)
	if err := json.Unmarshal(policyBytes, values); err != nil {
		return sanitizederror.NewWithError("failed to decode yaml", err)
	}

	if tf.enabled {
		var filteredResults []api.TestResults
		for _, res := range values.Results {
			if (len(tf.policy) == 0 || tf.policy == res.Policy) && (len(tf.resource) == 0 || tf.resource == res.Resource) && (len(tf.rule) == 0 || tf.rule == res.Rule) {
				filteredResults = append(filteredResults, res)
			}
		}
		values.Results = filteredResults
	}
	if len(values.Results) == 0 {
		return nil
	}

	fmt.Printf("\nExecuting %s...\n", values.Name)
	valuesFile := values.Variables
	userInfoFile := values.UserInfo

	variables, globalValMap, valuesMap, namespaceSelectorMap, subresources, err := common.GetVariable(variablesString, values.Variables, fs, isGit, policyResourcePath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return err
	}

	// get the user info as request info from a different file
	var userInfo v1beta1.RequestInfo

	if userInfoFile != "" {
		userInfo, err = common.GetUserInfoFromPath(fs, userInfoFile, isGit, policyResourcePath)
		if err != nil {
			fmt.Printf("Error: failed to load request info\nCause: %s\n", err)
			os.Exit(1)
		}
	}

	policyFullPath := getFullPath(values.Policies, policyResourcePath, isGit)
	resourceFullPath := getFullPath(values.Resources, policyResourcePath, isGit)

	for i, result := range values.Results {
		arrPatchedResource := []string{result.PatchedResource}
		arrGeneratedResource := []string{result.GeneratedResource}
		arrCloneSourceResource := []string{result.CloneSourceResource}

		patchedResourceFullPath := getFullPath(arrPatchedResource, policyResourcePath, isGit)
		generatedResourceFullPath := getFullPath(arrGeneratedResource, policyResourcePath, isGit)
		CloneSourceResourceFullPath := getFullPath(arrCloneSourceResource, policyResourcePath, isGit)

		values.Results[i].PatchedResource = patchedResourceFullPath[0]
		values.Results[i].GeneratedResource = generatedResourceFullPath[0]
		values.Results[i].CloneSourceResource = CloneSourceResourceFullPath[0]
	}

	policies, err := common.GetPoliciesFromPaths(fs, policyFullPath, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	var filteredPolicies []kyvernov1.PolicyInterface
	for _, p := range policies {
		for _, res := range values.Results {
			if p.GetName() == res.Policy {
				filteredPolicies = append(filteredPolicies, p)
				break
			}
		}
	}

	ruleToCloneSourceResource := map[string]string{}
	for _, p := range filteredPolicies {
		var filteredRules []kyvernov1.Rule

		for _, rule := range autogen.ComputeRules(p) {
			for _, res := range values.Results {
				if rule.Name == res.Rule {
					filteredRules = append(filteredRules, rule)
					if rule.HasGenerate() {
						ruleUnstr, err := generate.GetUnstrRule(rule.Generation.DeepCopy())
						if err != nil {
							fmt.Printf("Error: failed to get unstructured rule\nCause: %s\n", err)
							break
						}

						genClone, _, err := unstructured.NestedMap(ruleUnstr.Object, "clone")
						if err != nil {
							fmt.Printf("Error: failed to read data\nCause: %s\n", err)
							break
						}

						if len(genClone) != 0 {
							ruleToCloneSourceResource[rule.Name] = res.CloneSourceResource
						}
					}
					break
				}
			}
		}
		p.GetSpec().SetRules(filteredRules)
	}
	policies = filteredPolicies

	err = common.PrintMutatedPolicy(policies)
	if err != nil {
		return sanitizederror.NewWithError("failed to print mutated policy", err)
	}

	resources, err := common.GetResourceAccordingToResourcePath(fs, resourceFullPath, false, policies, dClient, "", false, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		os.Exit(1)
	}

	checkableResources := selectResourcesForCheck(resources, values)

	msgPolicies := "1 policy"
	if len(policies) > 1 {
		msgPolicies = fmt.Sprintf("%d policies", len(policies))
	}

	msgResources := "1 resource"
	if len(checkableResources) > 1 {
		msgResources = fmt.Sprintf("%d resources", len(checkableResources))
	}

	if len(policies) > 0 && len(checkableResources) > 0 {
		fmt.Printf("applying %s to %s... \n", msgPolicies, msgResources)
	}

	for _, policy := range policies {
		_, err := policyvalidation.Validate(policy, nil, nil, true, openApiManager, config.KyvernoUserName(config.KyvernoServiceAccountName()))
		if err != nil {
			log.Log.Error(err, "skipping invalid policy", "name", policy.GetName())
			continue
		}

		matches := common.HasVariables(policy)
		variable := common.RemoveDuplicateAndObjectVariables(matches)

		if len(variable) > 0 {
			if len(variables) == 0 {
				// check policy in variable file
				if valuesFile == "" || valuesMap[policy.GetName()] == nil {
					fmt.Printf("test skipped for policy  %v  (as required variables are not provided by the users) \n \n", policy.GetName())
				}
			}
		}

		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, subresources, dClient)

		for _, resource := range checkableResources {
			thisPolicyResourceValues, err := common.CheckVariableForPolicy(valuesMap, globalValMap, policy.GetName(), resource.GetName(), resource.GetKind(), variables, kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy.GetName(), resource.GetName()), err)
			}
			applyPolicyConfig := common.ApplyPolicyConfig{
				Policy:                    policy,
				Resource:                  resource,
				MutateLogPath:             "",
				Variables:                 thisPolicyResourceValues,
				UserInfo:                  userInfo,
				PolicyReport:              true,
				NamespaceSelectorMap:      namespaceSelectorMap,
				Rc:                        &resultCounts,
				RuleToCloneSourceResource: ruleToCloneSourceResource,
				Client:                    dClient,
				Subresources:              subresources,
			}
			ers, err := common.ApplyPolicyOnResource(applyPolicyConfig)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			engineResponses = append(engineResponses, ers...)
		}
	}
	resultsMap, testResults := buildPolicyResults(engineResponses, values.Results, policyResourcePath, fs, isGit, auditWarn)
	resultErr := printTestResult(resultsMap, testResults, rc, failOnly, removeColor)
	if resultErr != nil {
		return sanitizederror.NewWithError("failed to print test result:", resultErr)
	}

	return
}

func selectResourcesForCheck(resources []*unstructured.Unstructured, values *api.Test) []*unstructured.Unstructured {
	res, _, _ := selectResourcesForCheckInternal(resources, values)
	return res
}

// selectResourcesForCheckInternal internal method to test duplicates and unused
func selectResourcesForCheckInternal(resources []*unstructured.Unstructured, values *api.Test) ([]*unstructured.Unstructured, int, int) {
	var duplicates int
	var unused int
	uniqResources := make(map[string]*unstructured.Unstructured)

	for i := range resources {
		r := resources[i]
		key := fmt.Sprintf("%s/%s/%s", r.GetKind(), r.GetName(), r.GetNamespace())
		if _, ok := uniqResources[key]; ok {
			fmt.Println("skipping duplicate resource, resource :", r)
			duplicates++
		} else {
			uniqResources[key] = r
		}
	}

	selectedResources := map[string]*unstructured.Unstructured{}
	for key := range uniqResources {
		r := uniqResources[key]
		for _, res := range values.Results {
			if res.Kind == r.GetKind() {
				for _, testr := range res.Resources {
					if r.GetName() == testr {
						selectedResources[key] = r
					}
				}
				if r.GetName() == res.Resource {
					selectedResources[key] = r
				}
			}
		}
	}

	var checkableResources []*unstructured.Unstructured

	for key := range selectedResources {
		checkableResources = append(checkableResources, selectedResources[key])
		delete(uniqResources, key)
	}
	for _, r := range uniqResources {
		fmt.Println("skipping unused resource, resource :", r)
		unused++
	}
	return checkableResources, duplicates, unused
}

func printTestResult(resps map[string]policyreportv1alpha2.PolicyReportResult, testResults []api.TestResults, rc *resultCounts, failOnly, removeColor bool) error {
	printer := newTablePrinter(removeColor)
	var table []Table

	var countDeprecatedResource int
	testCount := 1
	for _, v := range testResults {
		res := new(Table)
		res.ID = testCount
		if v.Resources == nil {
			testCount++
		}
		res.Policy = colorize(removeColor, boldFgCyan, v.Policy)
		res.Rule = colorize(removeColor, boldFgCyan, v.Rule)

		if v.Resources != nil {
			for _, resource := range v.Resources {
				res.ID = testCount
				testCount++
				res.Resource = colorize(removeColor, boldFgCyan, v.Namespace) + "/" + colorize(removeColor, boldFgCyan, v.Kind) + "/" + colorize(removeColor, boldFgCyan, resource)
				var ruleNameInResultKey string
				if v.AutoGeneratedRule != "" {
					ruleNameInResultKey = fmt.Sprintf("%s-%s", v.AutoGeneratedRule, v.Rule)
				} else {
					ruleNameInResultKey = v.Rule
				}

				resultKey := fmt.Sprintf("%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Kind, resource)
				found, _ := isNamespacedPolicy(v.Policy)
				var ns string
				ns, v.Policy = getUserDefinedPolicyNameAndNamespace(v.Policy)
				if found && v.Namespace != "" {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, resource)
				} else if found {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Kind, resource)
					res.Policy = colorize(removeColor, boldFgCyan, ns) + "/" + colorize(removeColor, boldFgCyan, v.Policy)
					res.Resource = colorize(removeColor, boldFgCyan, v.Namespace) + "/" + colorize(removeColor, boldFgCyan, v.Kind) + "/" + colorize(removeColor, boldFgCyan, resource)
				} else if v.Namespace != "" {
					res.Resource = colorize(removeColor, boldFgCyan, v.Namespace) + "/" + colorize(removeColor, boldFgCyan, v.Kind) + "/" + colorize(removeColor, boldFgCyan, resource)
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, resource)
				}

				var testRes policyreportv1alpha2.PolicyReportResult
				if val, ok := resps[resultKey]; ok {
					testRes = val
				} else {
					log.Log.V(2).Info("result not found", "key", resultKey)
					res.Result = colorize(removeColor, boldYellow, "Not found")
					rc.Fail++
					table = append(table, *res)
					ftable = append(ftable, *res)
					continue
				}

				if v.Result == "" && v.Status != "" {
					v.Result = v.Status
				}

				if testRes.Result == v.Result {
					res.Result = colorize(removeColor, boldGreen, "Pass")
					if testRes.Result == policyreportv1alpha2.StatusSkip {
						rc.Skip++
					} else {
						rc.Pass++
					}
				} else {
					log.Log.V(2).Info("result mismatch", "expected", v.Result, "received", testRes.Result, "key", resultKey)
					res.Result = colorize(removeColor, boldRed, "Fail")
					rc.Fail++
					ftable = append(ftable, *res)
				}

				if failOnly {
					if res.Result == boldRed.Sprintf("Fail") || res.Result == "Fail" {
						table = append(table, *res)
					}
				} else {
					table = append(table, *res)
				}
			}
		} else if v.Resource != "" {
			countDeprecatedResource++
			res.Resource = colorize(removeColor, boldFgCyan, v.Namespace) + "/" + colorize(removeColor, boldFgCyan, v.Kind) + "/" + colorize(removeColor, boldFgCyan, v.Resource)
			var ruleNameInResultKey string
			if v.AutoGeneratedRule != "" {
				ruleNameInResultKey = fmt.Sprintf("%s-%s", v.AutoGeneratedRule, v.Rule)
			} else {
				ruleNameInResultKey = v.Rule
			}

			resultKey := fmt.Sprintf("%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Kind, v.Resource)
			found, _ := isNamespacedPolicy(v.Policy)
			var ns string
			ns, v.Policy = getUserDefinedPolicyNameAndNamespace(v.Policy)
			if found && v.Namespace != "" {
				resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, v.Resource)
			} else if found {
				resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Kind, v.Resource)
				res.Policy = colorize(removeColor, boldFgCyan, ns) + "/" + colorize(removeColor, boldFgCyan, v.Policy)
				res.Resource = colorize(removeColor, boldFgCyan, v.Namespace) + "/" + colorize(removeColor, boldFgCyan, v.Kind) + "/" + colorize(removeColor, boldFgCyan, v.Resource)
			} else if v.Namespace != "" {
				res.Resource = colorize(removeColor, boldFgCyan, v.Namespace) + "/" + colorize(removeColor, boldFgCyan, v.Kind) + "/" + colorize(removeColor, boldFgCyan, v.Resource)
				resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, v.Resource)
			}

			var testRes policyreportv1alpha2.PolicyReportResult
			if val, ok := resps[resultKey]; ok {
				testRes = val
			} else {
				log.Log.V(2).Info("result not found", "key", resultKey)
				res.Result = colorize(removeColor, boldYellow, "Not found")
				rc.Fail++
				table = append(table, *res)
				ftable = append(ftable, *res)
				continue
			}

			if v.Result == "" && v.Status != "" {
				v.Result = v.Status
			}

			if testRes.Result == v.Result {
				res.Result = colorize(removeColor, boldGreen, "Pass")
				if testRes.Result == policyreportv1alpha2.StatusSkip {
					rc.Skip++
				} else {
					rc.Pass++
				}
			} else {
				log.Log.V(2).Info("result mismatch", "expected", v.Result, "received", testRes.Result, "key", resultKey)
				res.Result = colorize(removeColor, boldRed, "Fail")
				rc.Fail++
				ftable = append(ftable, *res)
			}

			if failOnly {
				if res.Result == boldRed.Sprintf("Fail") || res.Result == "Fail" {
					table = append(table, *res)
				}
			} else {
				table = append(table, *res)
			}
		}
	}
	fmt.Printf("\n")
	printer.Print(table)
	return nil
}

func printFailedTestResult(removeColor bool) {
	printer := newTablePrinter(removeColor)
	for i, v := range ftable {
		v.ID = i + 1
	}
	fmt.Printf("Aggregated Failed Test Cases : ")
	fmt.Printf("\n")
	printer.Print(ftable)
}
