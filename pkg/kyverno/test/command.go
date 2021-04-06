package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kataras/tablewriter"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/openapi"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/lensesio/tableprinter"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// Command returns version command
func Command() *cobra.Command {
	var cmd *cobra.Command
	var valuesFile, fileName string
	cmd = &cobra.Command{
		Use:   "test",
		Short: "run tests from directory",
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

type SkippedPolicy struct {
	Name     string    `json:"name"`
	Rules    []v1.Rule `json:"rules"`
	Variable string    `json:"variable"`
}

type TestResults struct {
	Policy   string `json:"policy"`
	Rule     string `json:"rule"`
	Status   string `json:"status"`
	Resource string `json:"resource"`
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
	Resource string `header:"test"`
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
	pass int
	fail int
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
		if len(pathElems) <= 2 {
			err := fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch", gitURL.Path)
			fmt.Printf("Error: failed to parse URL \nCause: %s\n", err)
			os.Exit(1)
		}
		gitURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
		repoURL := gitURL.String()
		branch := strings.ReplaceAll(dirPath[0], repoURL+"/", "")
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
			if strings.Contains(file.Name(), fileName) {
				testYamlCount++
				policyresoucePath := strings.Trim(yamlFilePath, fileName)
				bytes, err := ioutil.ReadAll(file)
				if err != nil {
					sanitizederror.NewWithError("Error: failed to read file", err)
					continue
				}
				policyBytes, err := yaml.ToJSON(bytes)
				if err != nil {
					sanitizederror.NewWithError("failed to convert to JSON", err)
					continue
				}
				if err := applyPoliciesFromPath(fs, policyBytes, valuesFile, true, policyresoucePath, rc); err != nil {
					return rc, sanitizederror.NewWithError("failed to apply test command", err)
				}
			}
			if err != nil {
				sanitizederror.NewWithError("Error: failed to open file", err)
				continue
			}
		}
	} else {
		path := filepath.Clean(dirPath[0])
		if err != nil {
			errors = append(errors, err)
		}
		err := getLocalDirTestFiles(fs, path, fileName, valuesFile, rc, testYamlCount)
		if err != nil {
			errors = append(errors, err)
		}
		if len(errors) > 0 && log.Log.V(1).Enabled() {
			fmt.Printf("ignoring errors: \n")
			for _, e := range errors {
				fmt.Printf("    %v \n", e.Error())
			}
		}
	}
	if rc.fail > 0 {
		os.Exit(1)
	}
	if testYamlCount == 0 {
		fmt.Printf("\n No test yamls available \n")
	}
	os.Exit(0)
	return rc, nil
}

func getLocalDirTestFiles(fs billy.Filesystem, path, fileName, valuesFile string, rc *resultCounts, testYamlCount int) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read %v: %v", path, err.Error())
	}
	for _, file := range files {
		if file.IsDir() {
			getLocalDirTestFiles(fs, filepath.Join(path, file.Name()), fileName, valuesFile, rc, testYamlCount)
			continue
		}
		if strings.Contains(file.Name(), fileName) {
			testYamlCount++
			yamlFile, err := ioutil.ReadFile(filepath.Join(path, file.Name()))
			if err != nil {
				sanitizederror.NewWithError("unable to read yaml", err)
				continue
			}
			valuesBytes, err := yaml.ToJSON(yamlFile)
			if err != nil {
				sanitizederror.NewWithError("failed to convert json", err)
				continue
			}
			if err := applyPoliciesFromPath(fs, valuesBytes, valuesFile, false, path, rc); err != nil {
				sanitizederror.NewWithError("failed to apply test command", err)
				continue
			}
		}
	}
	return nil
}

func buildPolicyResults(resps []*response.EngineResponse) map[string][]interface{} {
	results := make(map[string][]interface{})
	infos := policyreport.GeneratePRsFromEngineResponse(resps, log.Log)
	for _, info := range infos {
		for _, infoResult := range info.Results {
			for _, rule := range infoResult.Rules {
				if rule.Type != utils.Validation.String() {
					continue
				}
				result := report.PolicyReportResult{
					Policy: info.PolicyName,
					Resources: []*corev1.ObjectReference{
						{
							Name: infoResult.Resource.Name,
						},
					},
				}
				result.Rule = rule.Name
				result.Status = report.PolicyStatus(rule.Check)
				results[rule.Name] = append(results[rule.Name], result)
			}
		}
	}
	return results
}

func getPolicyResouceFullPath(path []string, policyresoucePath string, isGit bool) []string {
	var pol []string
	if !isGit {
		for _, p := range path {
			pol = append(pol, filepath.Join(policyresoucePath, p))
		}
		return pol
	}
	return path
}

func applyPoliciesFromPath(fs billy.Filesystem, policyBytes []byte, valuesFile string, isGit bool, policyresoucePath string, rc *resultCounts) (err error) {
	openAPIController, err := openapi.NewOpenAPIController()
	engineResponses := make([]*response.EngineResponse, 0)
	validateEngineResponses := make([]*response.EngineResponse, 0)
	skippedPolicies := make([]SkippedPolicy, 0)
	var dClient *client.Client
	values := &Test{}
	var variablesString string

	if err := json.Unmarshal(policyBytes, values); err != nil {
		return sanitizederror.NewWithError("failed to decode yaml", err)
	}

	fmt.Printf("\nExecuting %s...", values.Name)

	_, valuesMap, contextVarMap, namespaceSelectorMap, err := common.GetVariable(variablesString, values.Variables, fs, isGit, policyresoucePath)
	fmt.Println("contextVarMap: ", contextVarMap)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return err
	}

	fullPolicyPath := getPolicyResouceFullPath(values.Policies, policyresoucePath, isGit)
	fullResourcePath := getPolicyResouceFullPath(values.Resources, policyresoucePath, isGit)

	policies, err := common.GetPoliciesFromPaths(fs, fullPolicyPath, isGit, policyresoucePath)
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
	resources, err := common.GetResourceAccordingToResourcePath(fs, fullResourcePath, false, mutatedPolicies, dClient, "", false, isGit, policyresoucePath)
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
	for _, policy := range mutatedPolicies {
		err := policy2.Validate(policy, nil, true, openAPIController)
		if err != nil {
			log.Log.V(3).Info(fmt.Sprintf("skipping policy %v as it is not valid", policy.Name), "error", err)
			continue
		}
		matches := common.PolicyHasVariables(*policy)
		variable := common.RemoveDuplicateVariables(matches)
		if len(matches) > 0 && variablesString == "" && values.Variables == "" {
			skipPolicy := SkippedPolicy{
				Name:     policy.GetName(),
				Rules:    policy.Spec.Rules,
				Variable: variable,
			}
			skippedPolicies = append(skippedPolicies, skipPolicy)
			log.Log.V(3).Info(fmt.Sprintf("skipping policy %s", policy.Name), "error", fmt.Sprintf("policy have variable - %s", variable))
			continue
		}
		for _, resource := range resources {
			thisPolicyResourceValues := make(map[string]string)
			if len(valuesMap[policy.GetName()]) != 0 && !reflect.DeepEqual(valuesMap[policy.GetName()][resource.GetName()], Resource{}) {
				thisPolicyResourceValues = valuesMap[policy.GetName()][resource.GetName()].Values
			}
			if len(common.PolicyHasVariables(*policy)) > 0 && len(thisPolicyResourceValues) == 0 {
				return sanitizederror.NewWithError(fmt.Sprintf("policy %s have variables. pass the values for the variables using set/values_file flag", policy.Name), err)
			}

			ers, validateErs, _, _, err := common.ApplyPolicyOnResource(policy, resource, "", false, thisPolicyResourceValues, true, namespaceSelectorMap, false)
			if err != nil {
				return sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
			}
			engineResponses = append(engineResponses, ers...)
			validateEngineResponses = append(validateEngineResponses, validateErs)
		}
	}
	resultsMap := buildPolicyResults(validateEngineResponses)
	resultErr := printTestResult(resultsMap, values.Results, rc)
	if resultErr != nil {
		return sanitizederror.NewWithError("Unable to genrate result. Error:", resultErr)
	}
	return
}

func printTestResult(resps map[string][]interface{}, testResults []TestResults, rc *resultCounts) error {
	printer := tableprinter.New(os.Stdout)
	table := []*Table{}
	boldRed := color.New(color.FgRed).Add(color.Bold)
	boldFgCyan := color.New(color.FgCyan).Add(color.Bold)
	for i, v := range testResults {
		res := new(Table)
		res.ID = i + 1
		res.Resource = boldFgCyan.Sprintf(v.Resource) + " with " + boldFgCyan.Sprintf(v.Policy) + "/" + boldFgCyan.Sprintf(v.Rule)
		n := resps[v.Rule]
		data, _ := json.Marshal(n)
		valuesBytes, err := yaml.ToJSON(data)
		if err != nil {
			return sanitizederror.NewWithError("failed to convert json", err)
		}
		var r []ReportResult
		json.Unmarshal(valuesBytes, &r)
		res.Result = boldRed.Sprintf("Fail")
		if len(r) != 0 {
			var resource TestResults
			for _, testRes := range r {
				if testRes.Resources[0].Name == v.Resource {
					resource.Policy = testRes.Policy
					resource.Rule = testRes.Rule
					resource.Status = testRes.Status
					resource.Resource = testRes.Resources[0].Name
					if v == resource {
						res.Result = "Pass"
						rc.pass++
					} else {
						rc.fail++
					}
				}
			}
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
	printer.Print(table)
	return nil
}
