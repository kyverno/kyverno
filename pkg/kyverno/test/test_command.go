package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-logr/logr"
	"github.com/kataras/tablewriter"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha2"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/generate"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/kyverno/store"
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
Test command provides a facility to test policies on resources. For that, user needs to provide the path of the folder containing test.yaml file. 

	kyverno test  /path/to/folderContaningTestYamls
				   or
	kyverno test  /path/to/githubRepository

The test.yaml file is configuration file for test command. It consists of 4 parts:-
  "policies"  (required) --> element lists one or more path of policies 
  "resources" (required) --> element lists one or more path of resources.
  "variables" (optional) --> element with one variables files
  "results"   (required)  --> element lists one more expected result. 
`
var exampleHelp = `
For Validate Policy 
  test.yaml

- name: test-1
  policies:
  - <path>
  - <path>
  resources:
  - <path>
  - <path>
  results:
  - policy: <name>
    rule: <name>
    resource: <name>
    namespace: <name> (OPTIONAL)
    kind: <name> 
    result: <pass/fail/skip>

For more visit --> https://kyverno.io/docs/kyverno-cli/#test


For Mutate Policy 
1) Policy (Namespaced-policy)

  test.yaml

- name: test-1
  policies:
  - <path>
  - <path>
  resources:
  - <path>
  - <path>
  results:
  - policy: <policy_namespace>/<policy_name>
    rule: <name>
    resource: <name>
    namespace: <name> (OPTIONAL)
    patchedResource: <path>
    kind: <name> 
    result: <pass/fail/skip>
  

2) ClusterPolicy(cluster-wide policy)

  test.yaml
  
- name: test-1
  policies:
  - <path>
  - <path>
  resources:
  - <path>
  - <path>
  results:
  - policy: <name>
    rule: <name>
    resource: <name>
    namespace: <name> (OPTIONAL)
    kind: <name> 
    patchedResource: <path>
    result: <pass/fail/skip>

NOTE:- 
In the results section, policy(if ClusterPolicy) or <policy_namespace>/<policy_name>(if Policy), rule, resource, kind and result are mandatory fields for all type of policy.

pass  --> patched Resource generated from engine equals to patched Resource provided by the user.
fail  --> patched Resource generated from engine is not equals to patched provided by the user. 
skip  --> rule is not applied.
`

// Command returns version command
func Command() *cobra.Command {
	var cmd *cobra.Command
	var valuesFile, fileName string
	cmd = &cobra.Command{
		Use:     "test",
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
			_, err = testCommandExecute(dirPath, valuesFile, fileName)
			if err != nil {
				log.Log.V(3).Info("a directory is required")
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "test.yaml", "test filename")
	return cmd
}

type Test struct {
	Name      string        `json:"name"`
	Policies  []string      `json:"policies"`
	Resources []string      `json:"resources"`
	Variables string        `json:"variables"`
	Results   []TestResults `json:"results"`
}

type TestResults struct {
	Policy            string              `json:"policy"`
	Rule              string              `json:"rule"`
	Result            report.PolicyResult `json:"result"`
	Status            report.PolicyResult `json:"status"`
	Resource          string              `json:"resource"`
	Kind              string              `json:"kind"`
	Namespace         string              `json:"namespace"`
	PatchedResource   string              `json:"patchedResource"`
	AutoGeneratedRule string              `json:"auto_generated_rule"`
}

type ReportResult struct {
	TestResults
	Resources []*corev1.ObjectReference `json:"resources"`
}

type Resource struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

type Table struct {
	ID       int    `header:"#"`
	Policy   string `header:"policy"`
	Rule     string `header:"rule"`
	Resource string `header:"resource"`
	Result   string `header:"result"`
}
type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources"`
}

type Values struct {
	Policies []Policy `json:"policies"`
}

type resultCounts struct {
	Skip int
	Pass int
	Fail int
}

func testCommandExecute(dirPath []string, valuesFile string, fileName string) (rc *resultCounts, err error) {
	var errors []error
	fs := memfs.New()
	rc = &resultCounts{}
	var testYamlCount int

	if len(dirPath) == 0 {
		return rc, sanitizederror.NewWithError(fmt.Sprintf("a directory is required"), err)
	}

	if strings.Contains(string(dirPath[0]), "https://") {
		gitURL, err := url.Parse(dirPath[0])
		if err != nil {
			return rc, sanitizederror.NewWithError("failed to parse URL", err)
		}

		pathElems := strings.Split(gitURL.Path[1:], "/")
		if len(pathElems) <= 1 {
			err := fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch", gitURL.Path)
			fmt.Printf("Error: failed to parse URL \nCause: %s\n", err)
			os.Exit(1)
		}

		gitURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
		repoURL := gitURL.String()
		branch := strings.ReplaceAll(dirPath[0], repoURL+"/", "")
		if branch == "" {
			branch = "main"
		}

		_, cloneErr := clone(repoURL, fs, branch)
		if cloneErr != nil {
			fmt.Printf("Error: failed to clone repository \nCause: %s\n", cloneErr)
			log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", cloneErr)
			os.Exit(1)
		}

		policyYamls, err := listYAMLs(fs, "/")
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

			if strings.Contains(file.Name(), fileName) {
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

				if err := applyPoliciesFromPath(fs, policyBytes, valuesFile, true, policyresoucePath, rc); err != nil {
					return rc, sanitizederror.NewWithError("failed to apply test command", err)
				}
			}
		}

		if testYamlCount == 0 {
			fmt.Printf("\n No test yamls available \n")
		}

	} else {
		path := filepath.Clean(dirPath[0])
		errors = getLocalDirTestFiles(fs, path, fileName, valuesFile, rc)
	}

	if len(errors) > 0 && log.Log.V(1).Enabled() {
		fmt.Printf("ignoring errors: \n")
		for _, e := range errors {
			fmt.Printf("    %v \n", e.Error())
		}
	}
	if rc.Fail > 0 {
		os.Exit(1)
	}
	os.Exit(0)
	return rc, nil
}

func getLocalDirTestFiles(fs billy.Filesystem, path, fileName, valuesFile string, rc *resultCounts) []error {
	var errors []error
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return []error{fmt.Errorf("failed to read %v: %v", path, err.Error())}
	}
	for _, file := range files {
		if file.IsDir() {
			getLocalDirTestFiles(fs, filepath.Join(path, file.Name()), fileName, valuesFile, rc)
			continue
		}
		if strings.Contains(file.Name(), fileName) {
			yamlFile, err := ioutil.ReadFile(filepath.Join(path, file.Name()))
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("unable to read yaml", err))
				continue
			}
			valuesBytes, err := yaml.ToJSON(yamlFile)
			if err != nil {
				errors = append(errors, sanitizederror.NewWithError("failed to convert json", err))
				continue
			}
			if err := applyPoliciesFromPath(fs, valuesBytes, valuesFile, false, path, rc); err != nil {
				errors = append(errors, sanitizederror.NewWithError(fmt.Sprintf("failed to apply test command from file %s", file.Name()), err))
				continue
			}
		}
	}
	return errors
}

func buildPolicyResults(resps []*response.EngineResponse, testResults []TestResults, infos []policyreport.Info, policyResourcePath string, fs billy.Filesystem, isGit bool) (map[string]report.PolicyReportResult, []TestResults) {
	results := make(map[string]report.PolicyReportResult)
	now := metav1.Timestamp{Seconds: time.Now().Unix()}

	for _, resp := range resps {
		policyName := resp.PolicyResponse.Policy.Name
		resourceName := resp.PolicyResponse.Resource.Name
		resourceKind := resp.PolicyResponse.Resource.Kind
		resourceNamespace := resp.PolicyResponse.Resource.Namespace
		policyNamespace := resp.PolicyResponse.Policy.Namespace

		var rules []string
		for _, rule := range resp.PolicyResponse.Rules {
			rules = append(rules, rule.Name)
		}
		result := report.PolicyReportResult{
			Policy: policyName,
			Resources: []*corev1.ObjectReference{
				{
					Name: resourceName,
				},
			},
		}
		var patcheResourcePath []string
		for i, test := range testResults {
			var userDefinedPolicyNamespace string
			var userDefinedPolicyName string
			found, _ := isNamespacedPolicy(test.Policy)
			if found {
				userDefinedPolicyNamespace, userDefinedPolicyName = getUserDefinedPolicyNameAndNamespace(test.Policy)
				test.Policy = userDefinedPolicyName
			}

			if test.Policy == policyName && test.Resource == resourceName {
				var resultsKey string
				resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource)
				if !util.ContainsString(rules, test.Rule) {
					if !util.ContainsString(rules, "autogen-"+test.Rule) {
						if !util.ContainsString(rules, "autogen-cronjob-"+test.Rule) {
							result.Result = report.StatusSkip
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
						result.Result = report.StatusSkip
						results[resultsKey] = result
					}
				}

				patcheResourcePath = append(patcheResourcePath, test.PatchedResource)

				if _, ok := results[resultsKey]; !ok {
					results[resultsKey] = result
				}
			}

		}

		for _, rule := range resp.PolicyResponse.Rules {
			if rule.Type != utils.Mutation.String() {
				continue
			}
			var resultsKey []string
			var resultKey string

			var result report.PolicyReportResult
			resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name, resourceNamespace, resourceKind, resourceName)
			for _, resultK := range resultsKey {
				if val, ok := results[resultK]; ok {
					result = val
					resultKey = resultK
				} else {
					continue
				}
				var x string
				for _, path := range patcheResourcePath {
					result.Result = report.StatusFail
					x = getAndComparePatchedResource(path, resp.PatchedResource, isGit, policyResourcePath, fs)
					if x == "pass" {
						result.Result = report.StatusPass
						break
					}
				}
				results[resultKey] = result
			}
		}
	}

	for _, info := range infos {
		for _, infoResult := range info.Results {
			for _, rule := range infoResult.Rules {
				if rule.Type != utils.Validation.String() {
					continue
				}

				var result report.PolicyReportResult
				var resultsKey []string
				var resultKey string
				resultsKey = GetAllPossibleResultsKey("", info.PolicyName, rule.Name, infoResult.Resource.Namespace, infoResult.Resource.Kind, infoResult.Resource.Name)
				for _, resultK := range resultsKey {
					if val, ok := results[resultK]; ok {
						result = val
						resultKey = resultK
					} else {
						continue
					}
				}

				result.Rule = rule.Name
				result.Result = report.PolicyResult(rule.Check)
				result.Source = policyreport.SourceValue
				result.Timestamp = now
				results[resultKey] = result
			}
		}
	}

	return results, testResults
}

func GetAllPossibleResultsKey(policyNamespace, policy, rule, namespace, kind, resource string) []string {
	var resultsKey []string
	resultKey1 := fmt.Sprintf("%s-%s-%s-%s", policy, rule, kind, resource)
	resultKey2 := fmt.Sprintf("%s-%s-%s-%s-%s", policy, rule, namespace, kind, resource)
	resultKey3 := fmt.Sprintf("%s-%s-%s-%s-%s", policyNamespace, policy, rule, kind, resource)
	resultKey4 := fmt.Sprintf("%s-%s-%s-%s-%s-%s", policyNamespace, policy, rule, namespace, kind, resource)
	resultsKey = append(resultsKey, resultKey1, resultKey2, resultKey3, resultKey4)
	return resultsKey
}

func GetResultKeyAccordingToTestResults(policyNs, policy, rule, namespace, kind, resource string) string {
	var resultKey string
	resultKey = fmt.Sprintf("%s-%s-%s-%s", policy, rule, kind, resource)

	if namespace != "" || policyNs != "" {
		if policyNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policyNs, policy, rule, kind, resource)
			if namespace != "" {
				resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", policyNs, policy, rule, namespace, kind, resource)
			}
		} else {
			resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policy, rule, namespace, kind, resource)
		}
	}
	return resultKey
}

func isNamespacedPolicy(policyNames string) (bool, error) {
	return regexp.MatchString("^[a-z]*/[a-z]*", policyNames)
}

func getUserDefinedPolicyNameAndNamespace(policyName string) (string, string) {
	policy := policyName
	policy_n_ns := strings.Split(policyName, "/")
	namespace := policy_n_ns[0]
	policy = policy_n_ns[1]
	return namespace, policy
}

// getAndComparePatchedResource --> Get the patchedResource from the path provided by user
// And compare this patchedResource with engine generated patcheResource.
func getAndComparePatchedResource(path string, enginePatchedResource unstructured.Unstructured, isGit bool, policyResourcePath string, fs billy.Filesystem) string {
	var status string
	patchedResources, err := common.GetPatchedResourceFromPath(fs, path, isGit, policyResourcePath)
	if err != nil {
		os.Exit(1)
	}
	var log logr.Logger
	matched, err := generate.ValidateResourceWithPattern(log, enginePatchedResource.UnstructuredContent(), patchedResources.UnstructuredContent())

	if err != nil {
		status = "fail"
	}
	if matched == "" {
		status = "pass"
	}
	return status
}

func getPolicyResourceFullPaths(path []string, policyResourcePath string, isGit bool) []string {
	var pol []string
	if !isGit {
		for _, p := range path {
			pol = append(pol, getPolicyResourceFullPath(p, policyResourcePath, isGit))
		}
		return pol
	}
	return path
}

func getPolicyResourceFullPath(path string, policyResourcePath string, isGit bool) string {
	var pol string
	if !isGit {
		pol = filepath.Join(policyResourcePath, path)

		return pol
	}
	return path
}

func applyPoliciesFromPath(fs billy.Filesystem, policyBytes []byte, valuesFile string, isGit bool, policyResourcePath string, rc *resultCounts) (err error) {
	openAPIController, err := openapi.NewOpenAPIController()
	engineResponses := make([]*response.EngineResponse, 0)
	var dClient *client.Client
	values := &Test{}
	var variablesString string
	var pvInfos []policyreport.Info
	var resultCounts common.ResultCounts
	store.SetMock(true)

	if err := json.Unmarshal(policyBytes, values); err != nil {
		return sanitizederror.NewWithError("failed to decode yaml", err)
	}

	fmt.Printf("\nExecuting %s...", values.Name)

	variables, valuesMap, namespaceSelectorMap, err := common.GetVariable(variablesString, values.Variables, fs, isGit, policyResourcePath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return err
	}

	fullPolicyPath := getPolicyResourceFullPaths(values.Policies, policyResourcePath, isGit)
	fullResourcePath := getPolicyResourceFullPaths(values.Resources, policyResourcePath, isGit)

	for i, result := range values.Results {
		var a []string
		a = append(a, result.PatchedResource)
		a = getPolicyResourceFullPaths(a, policyResourcePath, isGit)
		values.Results[i].PatchedResource = a[0]
	}
	policies, err := common.GetPoliciesFromPaths(fs, fullPolicyPath, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	mutatedPolicies, err := common.MutatePolices(policies)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return sanitizederror.NewWithError("failed to mutate policy", err)
		}
	}

	err = common.PrintMutatedPolicy(mutatedPolicies)
	if err != nil {
		return sanitizederror.NewWithError("failed to print mutated policy", err)
	}

	resources, err := common.GetResourceAccordingToResourcePath(fs, fullResourcePath, false, mutatedPolicies, dClient, "", false, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		os.Exit(1)
	}

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

	if variablesString != "" {
		variables = common.SetInStoreContext(mutatedPolicies, variables)
	}
	for _, policy := range mutatedPolicies {
		err := policy2.Validate(policy, nil, true, openAPIController)
		if err != nil {
			log.Log.Error(err, "skipping invalid policy", "name", policy.Name)
			continue
		}

		matches := common.PolicyHasVariables(*policy)
		variable := common.RemoveDuplicateAndObjectVariables(matches)

		if len(variable) > 0 {
			if len(variables) == 0 {
				// check policy in variable file
				if valuesFile == "" || valuesMap[policy.Name] == nil {
					fmt.Printf("test skipped for policy  %v  (as required variables are not provided by the users) \n \n", policy.Name)
				}
			}
		}

		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy)

		for _, resource := range resources {
			thisPolicyResourceValues, err := common.CheckVariableForPolicy(valuesMap, policy.GetName(), resource.GetName(), resource.GetKind(), variables, kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy.Name, resource.GetName()), err)
			}

			ers, info, err := common.ApplyPolicyOnResource(policy, resource, "", false, thisPolicyResourceValues, true, namespaceSelectorMap, false, &resultCounts, false)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
			}
			engineResponses = append(engineResponses, ers)
			pvInfos = append(pvInfos, info)
		}
	}
	resultsMap, testResults := buildPolicyResults(engineResponses, values.Results, pvInfos, policyResourcePath, fs, isGit)
	resultErr := printTestResult(resultsMap, testResults, rc)
	if resultErr != nil {
		return sanitizederror.NewWithError("Unable to genrate result. Error:", resultErr)
	}
	return
}

func printTestResult(resps map[string]report.PolicyReportResult, testResults []TestResults, rc *resultCounts) error {
	printer := tableprinter.New(os.Stdout)
	table := []*Table{}
	boldGreen := color.New(color.FgGreen).Add(color.Bold)
	boldRed := color.New(color.FgRed).Add(color.Bold)
	boldYellow := color.New(color.FgYellow).Add(color.Bold)
	boldFgCyan := color.New(color.FgCyan).Add(color.Bold)
	for i, v := range testResults {
		res := new(Table)
		res.ID = i + 1
		res.Policy = boldFgCyan.Sprintf(v.Policy)
		res.Rule = boldFgCyan.Sprintf(v.Rule)
		namespace := "default"
		if v.Namespace != "" {
			namespace = v.Namespace
		}
		res.Resource = boldFgCyan.Sprintf(namespace) + "/" + boldFgCyan.Sprintf(v.Kind) + "/" + boldFgCyan.Sprintf(v.Resource)
		var ruleNameInResultKey string
		if v.AutoGeneratedRule != "" {
			ruleNameInResultKey = fmt.Sprintf("%s-%s", v.AutoGeneratedRule, v.Rule)
		} else {
			ruleNameInResultKey = v.Rule
		}
		resultKey := fmt.Sprintf("%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Kind, v.Resource)
		found, _ := isNamespacedPolicy(v.Policy)
		if found || v.Namespace != "" {
			if found {
				var ns string
				ns, v.Policy = getUserDefinedPolicyNameAndNamespace(v.Policy)
				resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Kind, v.Resource)
				res.Policy = boldFgCyan.Sprintf(ns) + "/" + boldFgCyan.Sprintf(v.Policy)
				res.Resource = boldFgCyan.Sprintf(namespace) + "/" + boldFgCyan.Sprintf(v.Kind) + "/" + boldFgCyan.Sprintf(v.Resource)
				if v.Namespace != "" {
					resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", ns, v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, v.Resource)
				}
			} else {
				res.Resource = boldFgCyan.Sprintf(namespace) + "/" + boldFgCyan.Sprintf(v.Kind) + "/" + boldFgCyan.Sprintf(v.Resource)
				resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", v.Policy, ruleNameInResultKey, v.Namespace, v.Kind, v.Resource)
			}
		}
		var testRes report.PolicyReportResult
		if val, ok := resps[resultKey]; ok {
			testRes = val
		} else {
			res.Result = boldYellow.Sprintf("Not found")
			rc.Fail++
			table = append(table, res)
			continue
		}
		if v.Result == "" && v.Status != "" {
			v.Result = v.Status
		}
		if testRes.Result == v.Result {
			if testRes.Result == report.StatusSkip {
				res.Result = boldYellow.Sprintf("Skip")
				rc.Skip++
			} else {
				res.Result = boldGreen.Sprintf("Pass")
				rc.Pass++
			}
		} else {
			res.Result = boldRed.Sprintf("Fail")
			rc.Fail++
		}
		table = append(table, res)
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
