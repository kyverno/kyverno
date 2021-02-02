package test 
import (
	"fmt"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"net/url"
	"sort"
	"reflect"
	"strings"
	"github.com/spf13/cobra"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	"k8s.io/apimachinery/pkg/util/yaml"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/engine/response"
	corev1 "k8s.io/api/core/v1"
	"github.com/kyverno/kyverno/pkg/policyreport"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kataras/tablewriter"
	"github.com/lensesio/tableprinter"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5"
	"github.com/fatih/color"
	client "github.com/kyverno/kyverno/pkg/dclient"
)

// Command returns version command
func Command() *cobra.Command {

	var  valuesFile string
	return &cobra.Command{
		Use:   "test",
		Short: "Shows current test of kyverno",
		RunE: func(cmd *cobra.Command, dirPath []string) (err error) {
			defer func() {
				if err != nil {
					if !sanitizederror.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")
					}
				}
			}()
			err = testCommandExecute(dirPath, valuesFile)
			if err != nil {
				log.Log.V(3).Info("a directory is required")
				return err
			}
			return nil
		},
	}
}

type Test struct {
	Name          string     `json:"name"`
	Policies      []string     `json:"policies"`
	Resources     []string     `json:"resources"`
	Variables     string     `json:"variables"`
	Results      []TestResults     `json:"results"`
}

type SkippedPolicy struct {
	Name     string    `json:"name"`
	Rules    []v1.Rule `json:"rules"`
	Variable string    `json:"variable"`
}

type TestResults struct {
	Policy    string    `json:"policy"`
	Rule  string `json:"rule"`
	Status string    `json:"status"`
	Resource string    `json:"resource"`	
}

type ReportResult struct {
	TestResults 
	Resources []*corev1.ObjectReference `json:"resources"`
}

type Resource struct {
	Name          string     `json:"name"`
	Values map[string]string `json:"values"`
}

type Table struct {
	ID    int    `header:"#"`
	Resource string    `header:"test"`
	Result string    `header:"result"`
	
}
type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources"`
}

type Values struct {
	Policies []Policy `json:"policies"`
}

func testCommandExecute(dirPath []string, valuesFile string) (err error) {
	var errors []error
	fs := memfs.New()
	
	if len(dirPath) == 0 {
			return  sanitizederror.NewWithError(fmt.Sprintf("a directory is required"), err)
		}
	if strings.Contains(string(dirPath[0]), "https://") {
		gitUrl, err := url.Parse(dirPath[0])
		if err != nil {
			return  sanitizederror.NewWithError("failed to parse URL", err)
		}
		pathElems := strings.Split(gitUrl.Path[1:], "/")
		if len(pathElems) != 3 {
			err := fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch", gitUrl.Path)
			return  sanitizederror.NewWithError("failed to parse URL", err)
		}
		gitUrl.Path = strings.Join([]string{"/", pathElems[0], pathElems[1]}, "/")
		repoURL := gitUrl.String()
		cloneRepo, err := clone(repoURL, fs)
		if err != nil {
			return  sanitizederror.NewWithError("failed to clone repository ", err)
		}
		log.Log.V(3).Info(" clone repository", cloneRepo )
		policyYamls, err := listYAMLs(fs, "/")
		if err != nil {
			return  sanitizederror.NewWithError("failed to list YAMLs in repository", err)
		}
		sort.Strings(policyYamls)
		for _, yamlFilePath := range policyYamls {
			file, err := fs.Open(yamlFilePath)	
			bytes, err := ioutil.ReadAll(file)
			if err != nil {
				sanitizederror.NewWithError("Error: failed to read file", err)
			}
			policyBytes, err := yaml.ToJSON(bytes)
			if err != nil {
				sanitizederror.NewWithError("failed to convert to JSON", err)
				continue
			}
			if err := applyPoliciesFromPath(fs, policyBytes, valuesFile, true); err != nil {
				return sanitizederror.NewWithError("failed to apply test command", err)
			}	
		}
	} else {
		path := filepath.Clean(dirPath[0])
		fileDesc, err := os.Stat(path)
		if err != nil {
			errors = append(errors, err)
		}
		if fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to read %v: %v", path, err.Error()))
			}
			for _, file := range files {
				fmt.Printf("\napplying  test on file  %s...", file.Name())

				yamlFile, err := ioutil.ReadFile(filepath.Join(path, file.Name()))
				if err != nil {
					return  sanitizederror.NewWithError("unable to read yaml", err)
				}
				valuesBytes, err := yaml.ToJSON(yamlFile)
				if err != nil {
					return  sanitizederror.NewWithError("failed to convert json", err)
				}
				if err := applyPoliciesFromPath(fs, valuesBytes, valuesFile, false); err != nil {
					return sanitizederror.NewWithError("failed to apply test command", err)
				}	
			}
		}
		if len(errors) > 0 && log.Log.V(1).Enabled() {
			fmt.Printf("ignoring errors: \n")
			for _, e := range errors {
				fmt.Printf("    %v \n", e.Error())
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
							Name:       infoResult.Resource.Name,
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


func applyPoliciesFromPath(fs billy.Filesystem, policyBytes []byte, valuesFile string, isGit bool) (err error) {
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
	_, valuesMap, err := common.GetVariable(variablesString, values.Variables)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return  sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return  err
	}
	policies, err := common.GetPoliciesFromPaths(fs, values.Policies, isGit)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}
	mutatedPolicies, err := common.MutatePolices(policies)
		if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return  sanitizederror.NewWithError("failed to mutate policy", err)
			}
		}
		resources, err := common.GetResourceAccordingToResourcePath(fs, values.Resources, false,  mutatedPolicies, dClient, "", false, isGit)
		if err != nil {
			fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
			os.Exit(1)
		}
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
			if len(matches) > 0 && valuesFile == "" {
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
					return  sanitizederror.NewWithError(fmt.Sprintf("policy %s have variables. pass the values for the variables using set/values_file flag", policy.Name), err)
				}
				
				ers, validateErs, _, _, err := common.ApplyPolicyOnResource(policy, resource, "", false, thisPolicyResourceValues, true)
				if err != nil {
					return  sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
				}
				engineResponses = append(engineResponses, ers...)
				validateEngineResponses = append(validateEngineResponses, validateErs)
			}
		}
		resultsMap := buildPolicyResults(validateEngineResponses)
		resuleErr := printTestResult(resultsMap, values.Results)
		if resuleErr != nil {
			return  sanitizederror.NewWithError("Unable to genrate result. error:", resuleErr)
			os.Exit(1)
		}
	return
}


func printTestResult(resps map[string][]interface {}, testResults []TestResults) (error){
	printer := tableprinter.New(os.Stdout)
	table := []*Table{}
	boldRed := color.New(color.FgRed).Add(color.Bold)
	boldFgCyan := color.New(color.FgCyan).Add(color.Bold)
	for i, v := range testResults {
		res := new(Table)
		res.ID = i+1
		res.Resource =  boldFgCyan.Sprintf(v.Resource) +  " with " + boldFgCyan.Sprintf(v.Policy)  + "/" +  boldFgCyan.Sprintf(v.Rule) 
		n := resps[v.Rule]
		data, _ := json.Marshal(n)
		valuesBytes, err := yaml.ToJSON(data)
		if err != nil {
			return  sanitizederror.NewWithError("failed to convert json", err)
		}
		var c []ReportResult
		json.Unmarshal(valuesBytes, &c)
		res.Result = boldRed.Sprintf("Fail")
		if len(c) != 0 {
			var resource1 TestResults
			for _, c1 := range c {
				if c1.Resources[0].Name == v.Resource {
				resource1.Policy = c1.Policy
				resource1.Rule = c1.Rule
				resource1.Status = c1.Status
				resource1.Resource = c1.Resources[0].Name
				
					if v == resource1 {
						res.Result =  "Pass"
					}
				}
			}
		table = append(table, res)
		}
	}	
	printer.BorderTop, printer.BorderBottom, printer.BorderLeft, printer.BorderRight = true, true, true, true
	printer.CenterSeparator = "│"
	printer.ColumnSeparator = "│"
	printer.RowSeparator = "─"
	printer.RowCharLimit = 300
	printer.RowLengthTitle  = func(rowsLength int) bool {
		return rowsLength > 10
	}
	printer.HeaderBgColor = tablewriter.BgBlackColor
	printer.HeaderFgColor = tablewriter.FgGreenColor
	printer.Print(table)
	return nil
}