package testing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kataras/tablewriter"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/openapi"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policyreport"
	util "github.com/kyverno/kyverno/pkg/utils"
	"github.com/lensesio/tableprinter"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	log "sigs.k8s.io/controller-runtime/pkg/log"
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

apiVersion: cli.kyverno.io/v1beta1
kind: KyvernoTest
metadata:
  name: <name>
  labels:            (OPTIONAL)
    <key>: <value>
  annotations:       (OPTIONAL)
    <key>: <value>
spec:
  policies:
  - <path/to/policy1.yaml>
  - <path/to/policy2.yaml>
  resources:
    <resource_pool_1>:
    - <path/to/resource1.yaml>
    - <path/to/resource2.yaml>
	<resource_pool_2>:
    - <path/to/resource3.yaml>
    - <path/to/resource4.yaml>
	<patchedResource_pool>:             (Only when mutate rule is defined)
    - <path/to/patched_resource1.yaml>
    - <path/to/patched_resource2.yaml>
	<generatedResource_pool>:           (Only when generate rule is defined)
    - <path/to/generated_resource1.yaml>
    - <path/to/generated_resource2.yaml>
	<cloneSourceResource_pool>:           (Only when generate rule is defined and need to clone resource)
    - <path/to/cloneSource_resource1.yaml>
    - <path/to/cloneSource_resource2.yaml>
  results:
  - policy: <name> (For Namespaced [Policy] files, format is <policy_namespace>/<policy_name>)
    rule: <name>
    resources:
    - old: <resource_pool_1:apiversion/group/<namespace>/<name>>
      patched: <patchedResource_pool:apiversion/group/<namespace>/<name>>
      cloneSource: <cloneSourceResource_pool:apiversion/group/<namespace>/<name>>
      generated: <generatedResource_pool:apiversion/group/<namespace>/<name>>
    namespace: <namespace>  (OPTIONAL)
    kind: <kind>
    result: <result>
  variables:                (OPTIONAL)
    global:
      <variable>: <value>
    policies:
    - name: <name>
      rules:
      - name: <name>
        values:
          <variable>: <value>
        namespaceSelector:
        - name: <name>
          labels:
		    <variable>: <value>
      resources:
      - name: <name>
        values:
		  <variable>: <value>
        userInfo:
          clusterRoles:
          - <value>

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
	var testFile []byte
	var fileName, gitBranch string
	var registryAccess bool
	cmd = &cobra.Command{
		Use: "testing <path_to_folder_Containing_test.yamls> [flags]\n  kyverno test <path_to_gitRepository_with_dir> --git-branch <branchName>\n  kyverno test --manifest-mutate > kyverno-test.yaml\n  kyverno test --manifest-validate > kyverno-test.yaml",
		// Args:    cobra.ExactArgs(1),
		Short:   "run tests from directory",
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

			mStatus, _ := cmd.Flags().GetBool("manifest-mutate")
			vStatus, _ := cmd.Flags().GetBool("manifest-validate")
			if mStatus {
				testFile = []byte(`apiVersion: cli.kyverno.io/v1beta1
kind: KyvernoTest
metadata:
  name: <name>
  labels:            (OPTIONAL)
    <key>: <value>
  annotations:       (OPTIONAL)
    <key>: <value>
spec:
  policies:
  - <path/to/policy1.yaml>
  - <path/to/policy2.yaml>
  resources:
    <resource_pool_1>:
    - <path/to/resource1.yaml>
    - <path/to/resource2.yaml>
	<resource_pool_2>:
    - <path/to/resource3.yaml>
    - <path/to/resource4.yaml>
	<patchedResource_pool>:            
    - <path/to/patched_resource1.yaml>
    - <path/to/patched_resource2.yaml>
  results:
  - policy: <name> (For Namespaced [Policy] files, format is <policy_namespace>/<policy_name>)
    rule: <name>
    resources:
    - object: <resource_pool_1:apiversion/group/<namespace>/<name>>
      patched: <patchedResource_pool:apiversion/group/<namespace>/<name>>
    namespace: <namespace>  (OPTIONAL)
    kind: <kind>
    result: <result>`)
				fmt.Println(string(testFile))
				return nil
			}
			if vStatus {
				testFile = []byte(`apiVersion: cli.kyverno.io/v1beta1
kind: KyvernoTest
metadata:
  name: <name>
  labels:            (OPTIONAL)
    <key>: <value>
  annotations:       (OPTIONAL)
    <key>: <value>
spec:
  policies:
  - <path/to/policy1.yaml>
  - <path/to/policy2.yaml>
  resources:
    <resource_pool_1>:
    - <path/to/resource1.yaml>
    - <path/to/resource2.yaml>
	<resource_pool_2>:
    - <path/to/resource3.yaml>
    - <path/to/resource4.yaml>
  results:
  - policy: <name> (For Namespaced [Policy] files, format is <policy_namespace>/<policy_name>)
    rule: <name>
    resources:
    - object: <resource_pool_1:apiversion/group/<namespace>/<name>>
    namespace: <namespace>  (OPTIONAL)
    kind: <kind>
    result: <result>`)
				fmt.Println(string(testFile))
				return nil
			}
			store.SetRegistryAccess(registryAccess)
			_, err = testCommandExecute(dirPath, fileName, gitBranch, testCase)
			if err != nil {
				log.Log.V(3).Info("a directory is required")
				return err
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "kyverno-test.yaml", "test filename")
	cmd.Flags().StringVarP(&gitBranch, "git-branch", "b", "", "test github repository branch")
	cmd.Flags().StringVarP(&testCase, "test-case-selector", "t", "", `run some specific test cases by passing a string argument in double quotes to this flag like - "policy=<policy_name>, rule=<rule_name>, resource=<resource_name". The argument could be any combination of policy, rule and resource.`)
	cmd.Flags().BoolP("manifest-mutate", "", false, "prints out a template test manifest for a mutate policy")
	cmd.Flags().BoolP("manifest-validate", "", false, "prints out a template test manifest for a validate policy")
	cmd.Flags().BoolVarP(&registryAccess, "registry", "", false, "If set to true, access the image registry using local docker credentials to populate external data")
	return cmd
}

type ReportResult struct {
	kyvernov1.Results
	Resources []*corev1.ObjectReference `json:"resources"`
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

var ftable = []Table{}

func Split(r rune) bool {
	return r == ':' || r == '/'
}

func testCommandExecute(dirPath []string, fileName string, gitBranch string, testCase string) (rc *resultCounts, err error) {
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

	openAPIController, err := openapi.NewOpenAPIController()
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

		_, cloneErr := clone(repoURL, fs, gitBranch)
		if cloneErr != nil {
			fmt.Printf("Error: failed to clone repository \nCause: %s\n", cloneErr)
			log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", cloneErr)
			os.Exit(1)
		}

		policyYamls, err := listYAMLs(fs, gitPathToYamls)
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
				bytes, err := ioutil.ReadAll(file)
				if err != nil {
					errors = append(errors, sanitizederror.NewWithError("Error: failed to read file", err))
					continue
				}

				policyBytes, err := yaml.ToJSON(bytes)
				if err != nil {
					errors = append(errors, sanitizederror.NewWithError("failed to convert to JSON", err))
					continue
				}
				if err := applyPoliciesFromPath(fs, policyBytes, true, policyresoucePath, rc, openAPIController, tf); err != nil {
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
		errors = getLocalDirTestFiles(fs, path, fileName, rc, &testFiles, openAPIController, tf)

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

	fmt.Printf("\nTest Summary: %d tests passed and %d tests failed\n", rc.Pass+rc.Skip, rc.Fail)
	fmt.Printf("\n")

	if rc.Fail > 0 {
		printFailedTestResult()
		os.Exit(1)
	}
	os.Exit(0)
	return rc, nil
}

func getLocalDirTestFiles(fs billy.Filesystem, path, fileName string, rc *resultCounts, testFiles *int, openAPIController *openapi.Controller, tf *testFilter) []error {
	var errors []error

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return []error{fmt.Errorf("failed to read %v: %v", path, err.Error())}
	}
	for _, file := range files {
		if file.IsDir() {
			getLocalDirTestFiles(fs, filepath.Join(path, file.Name()), fileName, rc, testFiles, openAPIController, tf)
			continue
		}
		if file.Name() == fileName {
			*testFiles++
			// We accept the risk of including files here as we read the test dir only.
			yamlFile, err := ioutil.ReadFile(filepath.Join(path, file.Name())) // #nosec G304
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("unable to read yaml", err))
				continue
			}
			valuesBytes, err := yaml.ToJSON(yamlFile)
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("failed to convert json", err))
				continue
			}
			if err := applyPoliciesFromPath(fs, valuesBytes, false, path, rc, openAPIController, tf); err != nil {
				errors = append(errors, sanitizederror.NewWithError(fmt.Sprintf("failed to apply test command from file %s", file.Name()), err))
				continue
			}
		}
	}
	return errors
}

func buildPolicyResults(engineResponses []*response.EngineResponse, testResults []kyvernov1.Results, infos []policyreport.Info, policyResourcePath string, fs billy.Filesystem, isGit bool, resourcesMap map[string][]*unstructured.Unstructured) (map[string]policyreportv1alpha2.PolicyReportResult, []kyvernov1.Results) {
	results := make(map[string]policyreportv1alpha2.PolicyReportResult)
	now := metav1.Timestamp{Seconds: time.Now().Unix()}

	for _, resp := range engineResponses {
		policyName := resp.PolicyResponse.Policy.Name
		resourceName := resp.PolicyResponse.Resource.Name
		resourceKind := resp.PolicyResponse.Resource.Kind
		resourceNamespace := resp.PolicyResponse.Resource.Namespace
		policyNamespace := resp.PolicyResponse.Policy.Namespace

		var rules []string
		for _, rule := range resp.PolicyResponse.Rules {
			rules = append(rules, rule.Name)
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
			gvk := strings.FieldsFunc(test.Kind, Split)
			found, err := isNamespacedPolicy(test.Policy)
			if err != nil {
				log.Log.V(3).Info("error when determining if policy is namespaced or not", "policy: ", test.Policy, "error: ", err)
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
						name := strings.FieldsFunc(resource.Object, Split)
						if name[len(name)-1] == resourceName {
							var resultsKey string
							resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, gvk[len(gvk)-1], name[len(name)-1])
							if !util.ContainsString(rules, test.Rule) {
								if !util.ContainsString(rules, "autogen-"+test.Rule) {
									if !util.ContainsString(rules, "autogen-cronjob-"+test.Rule) {
										result.Result = policyreportv1alpha2.StatusSkip
									} else {
										testResults[i].AutoGeneratedRule = "autogen-cronjob"
										test.Rule = "autogen-cronjob-" + test.Rule
										resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, gvk[len(gvk)-1], name[len(name)-1])
									}
								} else {
									testResults[i].AutoGeneratedRule = "autogen"
									test.Rule = "autogen-" + test.Rule
									resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, gvk[len(gvk)-1], name[len(name)-1])
								}

								if results[resultsKey].Result == "" {
									result.Result = policyreportv1alpha2.StatusSkip
									results[resultsKey] = result
								}
							}

							patchedResourcePath = append(patchedResourcePath, resource.Patched)
							if _, ok := results[resultsKey]; !ok {
								results[resultsKey] = result
							}
						}
					}
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.Type != response.Generation || test.Rule != rule.Name {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name, resourceNamespace, resourceKind, resourceName)
				for _, key := range resultsKey {
					for _, resource := range test.Resources {
						if val, ok := results[key]; ok {
							result = val
							resultKey = key
						} else {
							continue
						}

						if rule.Status == response.RuleStatusSkip {
							result.Result = policyreportv1alpha2.StatusSkip
						} else if rule.Status == response.RuleStatusError {
							result.Result = policyreportv1alpha2.StatusError
						} else {
							var x string
							name := strings.FieldsFunc(resource.Generated, Split)
							for _, r := range resourcesMap["generatedResource_pool"] {
								if gvk[len(gvk)-1] == r.GetKind() && test.Namespace == r.GetNamespace() && name[len(name)-1] == r.GetName() {
									if len(gvk) == 3 && r.GroupVersionKind().Group == gvk[0] {
										result.Result = policyreportv1alpha2.StatusFail
										x = getAndCompareResource(r, rule.GeneratedResource, isGit, policyResourcePath, fs, true, resourcesMap)
										if x == "pass" {
											result.Result = policyreportv1alpha2.StatusPass
											break
										}
									} else if len(gvk) == 2 {
										result.Result = policyreportv1alpha2.StatusFail
										x = getAndCompareResource(r, rule.GeneratedResource, isGit, policyResourcePath, fs, true, resourcesMap)
										if x == "pass" {
											result.Result = policyreportv1alpha2.StatusPass
											break
										}
									}
								}
							}
						}
						results[resultKey] = result
					}
				}
			}
		}

		for _, rule := range resp.PolicyResponse.Rules {
			if rule.Type != response.Mutation {
				continue
			}

			var resultsKey []string
			var resultKey string
			var result policyreportv1alpha2.PolicyReportResult
			resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name, resourceNamespace, resourceKind, resourceName)
			for _, key := range resultsKey {
				if val, ok := results[key]; ok {
					result = val
					resultKey = key
				} else {
					continue
				}

				if rule.Status == response.RuleStatusSkip {
					result.Result = policyreportv1alpha2.StatusSkip
				} else if rule.Status == response.RuleStatusError {
					result.Result = policyreportv1alpha2.StatusError
				} else {
					var x string
					for _, path := range patchedResourcePath {
						name := strings.FieldsFunc(path, Split)
						for _, r := range resourcesMap["patchedResource_pool"] {
							if name[len(name)-1] == r.GetName() {
								// if len(gvk) == 3 {
								// 	if r.GroupVersionKind().Group == name[3] {
								result.Result = policyreportv1alpha2.StatusFail
								x = getAndCompareResource(r, resp.PatchedResource, isGit, policyResourcePath, fs, false, resourcesMap)
								if x == "pass" {
									result.Result = policyreportv1alpha2.StatusPass
									break
								}
								//}
								//}
								// 	 else if len(name) == 5 {
								// 		result.Result = policyreportv1alpha2.StatusFail
								// 		x = getAndCompareResource(r, resp.PatchedResource, isGit, policyResourcePath, fs, false, resourcesMap)
								// 		if x == "pass" {
								// 			result.Result = policyreportv1alpha2.StatusPass
								// 			break
								// 		}
								// 	}
							}
						}
					}
				}

				results[resultKey] = result
			}
		}
	}

	for _, info := range infos {
		for _, infoResult := range info.Results {
			for _, rule := range infoResult.Rules {
				if rule.Type != string(response.Validation) && rule.Type != string(response.ImageVerify) {
					continue
				}

				var result policyreportv1alpha2.PolicyReportResult
				var resultsKeys []string
				var resultKey string
				resultsKeys = GetAllPossibleResultsKey("", info.PolicyName, rule.Name, infoResult.Resource.Namespace, infoResult.Resource.Kind, infoResult.Resource.Name)
				for _, key := range resultsKeys {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}
				}

				result.Rule = rule.Name
				result.Result = policyreportv1alpha2.PolicyResult(rule.Status)
				result.Source = policyreport.SourceValue
				result.Timestamp = now
				results[resultKey] = result
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
func getAndCompareResource(r *unstructured.Unstructured, engineResource unstructured.Unstructured, isGit bool, policyResourcePath string, fs billy.Filesystem, isGenerate bool, resourcesMap map[string][]*unstructured.Unstructured) string {
	var status string
	resourceType := "patchedResource"
	if isGenerate {
		resourceType = "generatedResource"
	}

	matched, err := generate.ValidateResourceWithPattern(log.Log, engineResource.UnstructuredContent(), r.UnstructuredContent())
	if err != nil {
		log.Log.V(3).Info(resourceType+" mismatch", "error", err.Error())
		status = "fail"
	}

	if matched == "" {
		status = "pass"
	}
	return status
}

func buildMessage(resp *response.EngineResponse) string {
	var bldr strings.Builder
	for _, ruleResp := range resp.PolicyResponse.Rules {
		fmt.Fprintf(&bldr, "  %s: %s \n", ruleResp.Name, ruleResp.Status.String())
		fmt.Fprintf(&bldr, "    %s \n", ruleResp.Message)
	}

	return bldr.String()
}

func getFullPath(paths []string, policyResourcePath string, isGit bool, resourceType string) []string {
	var pols []string
	var pol string
	if !isGit {
		for _, path := range paths {
			patharr := strings.FieldsFunc(path, Split)
			if len(patharr) > 0 {
				if patharr[0] == "home" {
					pol = path
				} else {
					pol = filepath.Join(policyResourcePath, path)
				}
			}
			_, err := ioutil.ReadFile(pol)
			if err != nil {
				fmt.Printf("failed to load %s: %s \nerror: %s\n", resourceType, path, err)
				os.Exit(1)
			}
			pols = append(pols, pol)
		}
		return pols
	}
	return paths
}

func GetVariables(values kyvernov1.Variables) (map[string]string, map[string]map[string]kyvernov1.Resourcev, map[string]map[string]string) {
	valuesMapResource := make(map[string]map[string]kyvernov1.Resourcev)
	valuesMapRule := make(map[string]map[string]kyvernov1.Rulev)
	namespaceSelectorMap := make(map[string]map[string]string)
	var globalValMap = make(map[string]string)
	reqObjVars := ""
	if values.Global == nil {
		values.Global = make(map[string]string)
		values.Global["request.operation"] = "CREATE"
		log.Log.V(3).Info("Defaulting request.operation to CREATE")
	} else {
		if val, ok := values.Global["request.operation"]; ok {
			if val == "" {
				values.Global["request.operation"] = "CREATE"
				log.Log.V(3).Info("Globally request.operation value provided by the user is empty, defaulting it to CREATE", "request.opearation: ", values.Global)
			}
		}
	}

	globalValMap = values.Global

	for _, p := range values.Policies {
		resourceMap := make(map[string]kyvernov1.Resourcev)
		for _, r := range p.Resources {
			if val, ok := r.Values["request.operation"]; ok {
				if val == "" {
					r.Values["request.operation"] = "CREATE"
					log.Log.V(3).Info("No request.operation found, defaulting it to CREATE", "policy", p.Name)
				}
			}
			for variableInFile := range r.Values {
				if strings.Contains(variableInFile, "request.object") {
					if !strings.Contains(reqObjVars, variableInFile) {
						reqObjVars = reqObjVars + "," + variableInFile
					}
					delete(r.Values, variableInFile)
					continue
				}
			}
			resourceMap[r.Name] = r
		}
		valuesMapResource[p.Name] = resourceMap

		if p.Rules != nil {
			ruleMap := make(map[string]kyvernov1.Rulev)
			for _, r := range p.Rules {
				ruleMap[r.Name] = r
				for _, n := range r.NamespaceSelector {
					namespaceSelectorMap[n.Name] = n.Labels
				}
			}
			valuesMapRule[p.Name] = ruleMap
		}
	}

	if reqObjVars != "" {
		fmt.Printf(("\nNOTICE: request.object.* variables are automatically parsed from the supplied resource. Ignoring value of variables `%v`.\n"), reqObjVars)
	}

	if globalValMap != nil {
		globalValMap["request.operation"] = "CREATE"
		log.Log.V(3).Info("Defaulting request.operation to CREATE")
	}

	storePolicies := make([]kyvernov1.Policies, 0)
	for _, p := range values.Policies {
		storeRules := make([]kyvernov1.Rulev, 0)
		for _, rule := range valuesMapRule[p.Name] {
			storeRules = append(storeRules, kyvernov1.Rulev{
				Name:          rule.Name,
				Values:        rule.Values,
				ForeachValues: rule.ForeachValues,
			})
		}
		storeResources := make([]kyvernov1.Resourcev, 0)
		for _, resource := range valuesMapResource[p.Name] {
			storeResources = append(storeResources, kyvernov1.Resourcev{
				Name:   resource.Name,
				Values: resource.Values,
			})
		}
		storePolicies = append(storePolicies, kyvernov1.Policies{
			Name:      p.Name,
			Rules:     storeRules,
			Resources: storeResources,
		})
	}

	store.SetContext(store.Context{
		Policies: storePolicies,
	})

	return globalValMap, valuesMapResource, namespaceSelectorMap
}

func GetUserInfoFromPath(values kyvernov1.Variables, policyBytes []byte, resource *unstructured.Unstructured) (kyvernov1beta1.RequestInfo, store.Subject, error) {
	userInfo := &kyvernov1beta1.RequestInfo{}
	subject := &store.Subject{}
	value := &kyvernov1.Test_manifest{}
	var errors []error
	var userinfo = kyvernov1.UserInfo{}

	if err := json.Unmarshal(policyBytes, value); err != nil {
		errors = append(errors, sanitizederror.NewWithError("failed to decode yaml", err))
	}
	if err := json.Unmarshal(policyBytes, subject); err != nil {
		errors = append(errors, sanitizederror.NewWithError("failed to decode yaml", err))
	}
	for _, p := range value.Spec.Variables.Policies {
		for _, r := range p.Resources {
			if r.Name == resource.GetName() {
				userinfo = r.UserInfo
			}
		}
	}

	b, err := json.Marshal(userinfo)
	if err != nil {
		errors = append(errors, sanitizederror.NewWithError("failed to decode yaml", err))
	}
	if err := json.Unmarshal(b, userInfo); err != nil {
		errors = append(errors, sanitizederror.NewWithError("failed to decode yaml", err))
	}
	if len(errors) > 0 && log.Log.V(1).Enabled() {
		fmt.Printf("ignoring errors: \n")
		for _, e := range errors {
			fmt.Printf("    %v \n", e.Error())
		}
	}
	return *userInfo, *subject, nil
}

func applyPoliciesFromPath(fs billy.Filesystem, policyBytes []byte, isGit bool, policyResourcePath string, rc *resultCounts, openAPIController *openapi.Controller, tf *testFilter) (err error) {
	engineResponses := make([]*response.EngineResponse, 0)
	var dClient dclient.Interface
	values := &kyvernov1.Test_manifest{}
	var pvInfos []policyreport.Info
	var resultCounts common.ResultCounts
	store.SetMock(true)

	if err := json.Unmarshal(policyBytes, values); err != nil {
		return sanitizederror.NewWithError("failed to decode yaml", err)
	}
	err1 := validation(values, isGit, policyResourcePath, string(policyBytes))
	if err1 != nil {
		fmt.Printf("Error : %q\n", err1)
		os.Exit(1)
	}

	if tf.enabled {
		var filteredResults []kyvernov1.Results
		for _, res := range values.Spec.Results {
			for _, resources := range res.Resources {
				if (len(tf.policy) == 0 || tf.policy == res.Policy) && (len(tf.resource) == 0 || tf.resource == resources.Object) && (len(tf.rule) == 0 || tf.rule == res.Rule) {
					filteredResults = append(filteredResults, res)
				}
			}
		}
		values.Spec.Results = filteredResults
	}
	if len(values.Spec.Results) == 0 {
		return nil
	}

	fmt.Printf("\nExecuting %s...", values.Metadata.Name)

	globalValMap, valuesMap, namespaceSelectorMap := GetVariables(values.Spec.Variables)
	resourceFullPath := make(map[string][]string)
	policyFullPath := getFullPath(values.Spec.Policies, policyResourcePath, isGit, "policy")
	for k := range values.Spec.Resources {
		resourceFullPath[k] = getFullPath(values.Spec.Resources[k], policyResourcePath, isGit, "resource")
	}

	policies, err := common.GetPoliciesFromPaths(fs, policyFullPath, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	filteredPolicies := []kyvernov1.PolicyInterface{}
	for _, p := range policies {
		for _, res := range values.Spec.Results {
			if p.GetName() == res.Policy {
				filteredPolicies = append(filteredPolicies, p)
				break
			}
		}
	}

	var ruleToCloneSourceResource = map[string]string{}
	for _, p := range filteredPolicies {
		filteredRules := []kyvernov1.Rule{}

		for _, rule := range autogen.ComputeRules(p) {
			for _, res := range values.Spec.Results {
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
						for _, resource := range res.Resources {
							if len(genClone) != 0 {
								ruleToCloneSourceResource[rule.Name] = resource.CloneSource
							}
						}
					}
					break
				}
			}
		}
		p.GetSpec().SetRules(filteredRules)
	}
	policies = filteredPolicies

	mutatedPolicies, err := common.MutatePolicies(policies)

	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return sanitizederror.NewWithError("failed to mutate policy", err)
		}
	}

	err = common.PrintMutatedPolicy(mutatedPolicies)
	if err != nil {
		return sanitizederror.NewWithError("failed to print mutated policy", err)
	}
	allresources := make(map[string][]*unstructured.Unstructured)
	resourcesMap := make(map[string][]*unstructured.Unstructured)
	var resources []*unstructured.Unstructured
	for k := range values.Spec.Resources {
		allresources[k], err = common.GetResourceAccordingToResourcePath(fs, resourceFullPath[k], false, mutatedPolicies, dClient, "", false, isGit, policyResourcePath)
		if err != nil {
			fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
			os.Exit(1)
		}
		resourcesMap[k] = allresources[k]
	}

	filteredResources := []*unstructured.Unstructured{}

	for _, res := range values.Spec.Results {
		for _, testr := range res.Resources {
			gvk := strings.FieldsFunc(res.Kind, Split)
			name := strings.FieldsFunc(testr.Object, Split)
			for _, r := range resourcesMap[name[0]] {
				if gvk[len(gvk)-2] == r.GroupVersionKind().Version && gvk[len(gvk)-1] == r.GetKind() && res.Namespace == r.GetNamespace() && name[len(name)-1] == r.GetName() {
					if len(gvk) == 3 {
						if r.GroupVersionKind().Group == gvk[0] {
							filteredResources = append(filteredResources, r)
						}
					} else if len(gvk) == 2 {
						filteredResources = append(filteredResources, r)
					}
				}
			}
		}
	}
	resources = filteredResources
	msgPolicies := "1 policy"
	if len(mutatedPolicies) > 1 {
		msgPolicies = fmt.Sprintf("%d policies", len(policies))
	}

	msgResources := "1 resource"
	if len(resources) > 1 {
		msgResources = fmt.Sprintf("%d resources", len(resources))
	}

	if len(mutatedPolicies) > 0 && len(resources) > 0 {
		fmt.Printf("\napplying %s to %s... \n", msgPolicies, msgResources)
	}

	for _, policy := range mutatedPolicies {
		_, err := policy2.Validate(policy, nil, true, openAPIController)
		if err != nil {
			fmt.Print("\n")
			fmt.Printf("policy : %v is not a valid policy\n", policy.GetName())
			log.Log.Error(err, "skipping invalid policy", "name", policy.GetName())
			os.Exit(1)
		}

		matches := common.HasVariables(policy)
		variable := common.RemoveDuplicateAndObjectVariables(matches)

		if len(variable) > 0 {
			// check policy in variable file
			if valuesMap[policy.GetName()] == nil {
				fmt.Printf("test skipped for policy  %v  (as required variables are not provided by the users) \n \n", policy.GetName())
				os.Exit(1)
			}
		}

		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy)
		for _, resource := range resources {
			thisPolicyResourceValues, err := CheckVariableForPolicyn(valuesMap, globalValMap, policy.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy.GetName(), resource.GetName()), err)
			}

			userInfo, subjectInfo, err := GetUserInfoFromPath(values.Spec.Variables, policyBytes, resource)
			if err != nil {
				fmt.Printf("Error: failed to load request info\nCause: %s\n", err)
				os.Exit(1)
			}
			store.SetSubjects(subjectInfo)
			ers, info, err := common.ApplyPolicyOnResource(resourcesMap, policy, resource, "", false, thisPolicyResourceValues, userInfo, true, namespaceSelectorMap, false, &resultCounts, false, ruleToCloneSourceResource)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			engineResponses = append(engineResponses, ers...)
			pvInfos = append(pvInfos, info)
		}
	}
	resultsMap, testResults := buildPolicyResults(engineResponses, values.Spec.Results, pvInfos, policyResourcePath, fs, isGit, resourcesMap)
	resultErr := printTestResult(resultsMap, testResults, rc)
	if resultErr != nil {
		return sanitizederror.NewWithError("failed to print test result:", resultErr)
	}

	return
}

func CheckVariableForPolicyn(valuesMap map[string]map[string]kyvernov1.Resourcev, globalValMap map[string]string, policyName string, resourceName string, resourceKind string, kindOnwhichPolicyIsApplied map[string]struct{}, variable string) (map[string]interface{}, error) {
	// get values from file for this policy resource combination
	thisPolicyResourceValues := make(map[string]interface{})
	if len(valuesMap[policyName]) != 0 && !reflect.DeepEqual(valuesMap[policyName][resourceName], kyvernov1.Resourcev{}) {
		thisPolicyResourceValues = valuesMap[policyName][resourceName].Values
	}

	if thisPolicyResourceValues == nil && len(globalValMap) > 0 {
		thisPolicyResourceValues = make(map[string]interface{})
	}

	for k, v := range globalValMap {
		if _, ok := thisPolicyResourceValues[k]; !ok {
			thisPolicyResourceValues[k] = v
		}
	}

	// skipping the variable check for non matching kind
	if _, ok := kindOnwhichPolicyIsApplied[resourceKind]; ok {
		if len(variable) > 0 && len(thisPolicyResourceValues) == 0 && len(store.GetContext().Policies) == 0 {
			return thisPolicyResourceValues, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policyName, resourceName), nil)
		}
	}
	return thisPolicyResourceValues, nil
}

func printTestResult(resps map[string]policyreportv1alpha2.PolicyReportResult, testResults []kyvernov1.Results, rc *resultCounts) error {
	printer := tableprinter.New(os.Stdout)
	table := []Table{}
	boldGreen := color.New(color.FgGreen).Add(color.Bold)
	boldRed := color.New(color.FgRed).Add(color.Bold)
	boldYellow := color.New(color.FgYellow).Add(color.Bold)
	boldFgCyan := color.New(color.FgCyan).Add(color.Bold)

	for i, v := range testResults {
		res := new(Table)
		res.ID = i + 1
		res.Policy = boldFgCyan.Sprintf(v.Policy)
		res.Rule = boldFgCyan.Sprintf(v.Rule)
		gvk := strings.FieldsFunc(v.Kind, Split)
		if v.Resources != nil {
			for _, resource := range v.Resources {
				name := strings.FieldsFunc(resource.Object, Split)
				res.Resource = boldFgCyan.Sprintf(v.Namespace) + "/" + boldFgCyan.Sprintf(gvk[len(gvk)-1]) + "/" + boldFgCyan.Sprintf(name[len(name)-1])
				var ruleNameInResultKey string
				if v.AutoGeneratedRule != "" {
					ruleNameInResultKey = fmt.Sprintf("%s-%s", v.AutoGeneratedRule, v.Rule)
				} else {
					ruleNameInResultKey = v.Rule
				}

				resultKey := fmt.Sprintf("%s-%s-%s-%s", v.Policy, ruleNameInResultKey, gvk[len(gvk)-1], name[len(name)-1])
				found, _ := isNamespacedPolicy(v.Policy)
				var ns string
				ns, v.Policy = getUserDefinedPolicyNameAndNamespace(v.Policy)
				if found && v.Namespace != "" {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Namespace, gvk[len(gvk)-1], name[len(name)-1])
				} else if found {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, gvk[len(gvk)-1], name[len(name)-1])
					res.Policy = boldFgCyan.Sprintf(ns) + "/" + boldFgCyan.Sprintf(v.Policy)
					res.Resource = boldFgCyan.Sprintf(v.Namespace) + "/" + boldFgCyan.Sprintf(gvk[len(gvk)-1]) + "/" + boldFgCyan.Sprintf(name[len(name)-1])
				} else if v.Namespace != "" {
					res.Resource = boldFgCyan.Sprintf(v.Namespace) + "/" + boldFgCyan.Sprintf(gvk[len(gvk)-1]) + "/" + boldFgCyan.Sprintf(name[len(name)-1])
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Namespace, gvk[len(gvk)-1], name[len(name)-1])
				}

				var testRes policyreportv1alpha2.PolicyReportResult
				if val, ok := resps[resultKey]; ok {
					testRes = val
				} else {
					log.Log.V(2).Info("result not found", "key", resultKey)
					res.Result = boldYellow.Sprintf("Not found")
					rc.Fail++
					table = append(table, *res)
					ftable = append(ftable, *res)
					continue
				}

				if string(testRes.Result) == v.Result {
					res.Result = boldGreen.Sprintf("Pass")
					if testRes.Result == policyreportv1alpha2.StatusSkip {
						res.Result = boldGreen.Sprintf("Pass")
						rc.Skip++
					} else {
						res.Result = boldGreen.Sprintf("Pass")
						rc.Pass++
					}
				} else {
					log.Log.V(2).Info("result mismatch", "expected", v.Result, "received", testRes.Result, "key", resultKey)
					res.Result = boldRed.Sprintf("Fail")
					rc.Fail++
					ftable = append(ftable, *res)
				}

				table = append(table, *res)
			}
		}
	}

	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.RowLengthTitle = func(rowsLength int) bool {
		return rowsLength > 10
	}

	printer.HeaderBgColor = tablewriter.BgBlackColor
	printer.HeaderFgColor = tablewriter.FgGreenColor
	fmt.Printf("\n")
	printer.Print(table)
	return nil
}
func printFailedTestResult() {
	printer := tableprinter.New(os.Stdout)
	for i, v := range ftable {
		v.ID = i + 1
	}
	fmt.Printf("Aggregated Failed Test Cases : ")
	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.RowLengthTitle = func(rowsLength int) bool {
		return rowsLength > 10
	}

	printer.HeaderBgColor = tablewriter.BgBlackColor
	printer.HeaderFgColor = tablewriter.FgGreenColor
	fmt.Printf("\n")
	printer.Print(ftable)
}
