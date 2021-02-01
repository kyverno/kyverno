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
	"bufio"
	"strings"
	"github.com/spf13/cobra"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/engine/response"
	yamlv2 "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"github.com/kyverno/kyverno/pkg/policyreport"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	ut "github.com/kyverno/kyverno/pkg/utils"
	"github.com/kataras/tablewriter"
	"github.com/lensesio/tableprinter"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5"
	"github.com/fatih/color"
)

// Command returns version command
func Command() *cobra.Command {

	var  valuesFile string
	return &cobra.Command{
		Use:   "test",
		Short: "Shows current test of kyverno",
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			defer func() {
				if err != nil {
					if !sanitizederror.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")
					}
				}
			}()

			err = testCommandHelper(policyPaths, valuesFile)
			if err != nil {
				return err
			}
			log.Log.V(3).Info("Test command fail to apply")
			return nil
		},
	}
}

type Test struct {
	Name          string     `json:"name"`
	Policies      []string     `json:"policies"`
	Resources     []string     `json:"resources"`
	Variables     string     `json:"variables"`
	TResults      []TestResults     `json:"results"`
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

func testCommandHelper(policyPaths []string, valuesFile string) (err error) {
	var errors []error
	fs := memfs.New()
	
	if len(policyPaths) == 0 {
			return  sanitizederror.NewWithError(fmt.Sprintf("require test yamls"), err)
		}
	if strings.Contains(string(policyPaths[0]), "https://github.com/") {
		u, err := url.Parse(policyPaths[0])
		if err != nil {
			return  sanitizederror.NewWithError("failed to parse URL", err)
		}
		pathElems := strings.Split(u.Path[1:], "/")
		if len(pathElems) != 3 {
			err := fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch", u.Path)
			return  sanitizederror.NewWithError("failed to parse URL", err)
		}
		u.Path = strings.Join([]string{"/", pathElems[0], pathElems[1]}, "/")
		repoURL := u.String()
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
		path := filepath.Clean(policyPaths[0])
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
func getPoliciesFromPaths(fs billy.Filesystem, policyPaths []string, isGit bool) (policies []*v1.ClusterPolicy, err error) {
	var errors []error
	if isGit {
		for _, pp := range policyPaths {
			filep, err := fs.Open(pp)
			bytes, err := ioutil.ReadAll(filep)
			if err != nil {
				fmt.Printf("Error: failed to read file %s: %v", filep.Name(), err.Error())
			}
			policyBytes, err := yaml.ToJSON(bytes)
			if err != nil {
				fmt.Printf("failed to convert to JSON: %v", err)
				continue
			}
			policiesFromFile, errFromFile := ut.GetPolicy(policyBytes)
			if errFromFile != nil {
				err := fmt.Errorf("failed to process : %v", errFromFile.Error())
				errors = append(errors, err)
				continue
			}
			policies = append(policies, policiesFromFile...)
		}
	} else {
		if len(policyPaths) > 0 && policyPaths[0] == "-" {
			if common.IsInputFromPipe() {
				policyStr := ""
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					policyStr = policyStr + scanner.Text() + "\n"
				}
	
				yamlBytes := []byte(policyStr)
				policies, err = ut.GetPolicy(yamlBytes)
				if err != nil {
					return nil, sanitizederror.NewWithError("failed to extract the resources", err)
				}
			}
		} else {
			var errors []error
			policies, errors = common.GetPolicies(policyPaths)
			if len(policies) == 0 {
				if len(errors) > 0 {
					return nil, sanitizederror.NewWithErrors("failed to read policies", errors)
				}
				return nil, sanitizederror.New(fmt.Sprintf("no policies found in paths %v", policyPaths))
			}
			if len(errors) > 0 && log.Log.V(1).Enabled() {
				fmt.Printf("ignoring errors: \n")
				for _, e := range errors {
					fmt.Printf("    %v \n", e.Error())
				}
			}
		}
	}	
	return
}

func mutatePolices(policies []*v1.ClusterPolicy) ([]*v1.ClusterPolicy, error) {
	newPolicies := make([]*v1.ClusterPolicy, 0)
	logger := log.Log.WithName("apply")

	for _, policy := range policies {
		p, err := common.MutatePolicy(policy, logger)
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return nil, sanitizederror.NewWithError("failed to mutate policy.", err)
			}
			return nil, err
		}
		newPolicies = append(newPolicies, p)
	}
	return newPolicies, nil
}
func getResourceAccordingToResourcePath(fs billy.Filesystem,resourcePaths []string,  policies []*v1.ClusterPolicy, isGit bool) (resources []*unstructured.Unstructured, err error) {
	resources, err = common.GetResourcesWithTest(fs, policies, resourcePaths, isGit)
	if err != nil {
		return resources, err
	}
	return resources, err
}
	
func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured) ([]*response.EngineResponse, *response.EngineResponse, error) {
	engineResponses := make([]*response.EngineResponse, 0)

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", policy.Name, "resource", resPath)

	ctx := context.NewContext()

	mutateResponse := engine.Mutate(&engine.PolicyContext{Policy: *policy, NewResource: *resource, JSONContext: ctx})
	engineResponses = append(engineResponses, mutateResponse)

	if !mutateResponse.IsSuccessful() {
		fmt.Printf("Failed to apply mutate policy %s -> resource %s", policy.Name, resPath)
		for i, r := range mutateResponse.PolicyResponse.Rules {
			fmt.Printf("\n%d. %s", i+1, r.Message)
		}
	} else {
		if len(mutateResponse.PolicyResponse.Rules) > 0 {
			yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
			if err != nil {
				log.Log.V(3).Info(fmt.Sprintf("yaml encoded resource not valid"), "error", err)
			}
			mutatedResource := string(yamlEncodedResource)
				if len(strings.TrimSpace(mutatedResource)) > 0 {
					fmt.Printf("\nmutate policy %s applied to %s:", policy.Name, resPath)
					fmt.Printf("\n" + mutatedResource)
					fmt.Printf("\n")
				}
		}
	}

	if resource.GetKind() == "Pod" && len(resource.GetOwnerReferences()) > 0 {
		if policy.HasAutoGenAnnotation() {
			if _, ok := policy.GetAnnotations()[engine.PodControllersAnnotation]; ok {
				delete(policy.Annotations, engine.PodControllersAnnotation)
			}
		}
	}
	policyCtx := &engine.PolicyContext{Policy: *policy, NewResource: mutateResponse.PatchedResource, JSONContext: ctx}
	validateResponse := engine.Validate(policyCtx)
	var policyHasGenerate bool
	for _, rule := range policy.Spec.Rules {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}
	if policyHasGenerate {
		generateResponse := engine.Generate(engine.PolicyContext{Policy: *policy, NewResource: *resource})
		engineResponses = append(engineResponses, generateResponse)
		if len(generateResponse.PolicyResponse.Rules) > 0 {
			log.Log.V(3).Info("generate resource is valid", "policy", policy.Name, "resource", resPath)
		} else {
			fmt.Printf("generate policy %s resource %s is invalid \n", policy.Name, resPath)
			for i, r := range generateResponse.PolicyResponse.Rules {
				fmt.Printf("%d. %s \b", i+1, r.Message)
			}

		}
	}
	

	return engineResponses, validateResponse, nil
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
func getVariable( valuesFile string) ( valuesMap map[string]map[string]Resource, err error) {
	if valuesFile != "" {
		yamlFile, err := ioutil.ReadFile(valuesFile)
		if err != nil {
			return valuesMap, sanitizederror.NewWithError("unable to read yaml", err)
		}
		valuesBytes, err := yaml.ToJSON(yamlFile)
		if err != nil {
			return valuesMap, sanitizederror.NewWithError("failed to convert json", err)
		}
		values := &Values{}
		if err := json.Unmarshal(valuesBytes, values); err != nil {
			return valuesMap, sanitizederror.NewWithError("failed to decode yaml", err)
		}
		for _, p := range values.Policies {
			pmap := make(map[string]Resource)
			for _, r := range p.Resources {
				pmap[r.Name] = r
			}
			valuesMap[p.Name] = pmap
		}
	}
	return  valuesMap, nil
}

func removeDuplicatevariables(matches [][]string) string {
	var variableStr string
	for _, m := range matches {
		for _, v := range m {
			foundVariable := strings.Contains(variableStr, v)
			if !foundVariable {
				variableStr = variableStr + " " + v
			}
		}
	}
	return variableStr
}

func applyPoliciesFromPath(fs billy.Filesystem, policyBytes []byte, valuesFile string, isGit bool) (err error) {
	openAPIController, err := openapi.NewOpenAPIController()
	engineResponses := make([]*response.EngineResponse, 0)
	validateEngineResponses := make([]*response.EngineResponse, 0)
	skippedPolicies := make([]SkippedPolicy, 0)
	values := &Test{}

	if err := json.Unmarshal(policyBytes, values); err != nil {
		return sanitizederror.NewWithError("failed to decode yaml", err)
	}	
	valuesMap, err := getVariable(values.Variables)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return  sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return  err
	}
	policies, err := getPoliciesFromPaths(fs, values.Policies, isGit)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}
	mutatedPolicies, err := mutatePolices(policies)
		if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return  sanitizederror.NewWithError("failed to mutate policy", err)
			}
		}
		resources, err := getResourceAccordingToResourcePath(fs,values.Resources, mutatedPolicies, isGit)
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
			variable := removeDuplicatevariables(matches)
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

				ers, validateErs, err := applyPolicyOnResource(policy, resource)
				if err != nil {
					return  sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
				}
				engineResponses = append(engineResponses, ers...)
				validateEngineResponses = append(validateEngineResponses, validateErs)
			}
		}
		resultsMap := buildPolicyResults(validateEngineResponses)
		resuleErr := printTestResult(resultsMap, values.TResults)
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
		if len(c) != 0 {
			var resource1 TestResults
			for _, c1 := range c {
				if c1.Resources[0].Name == v.Resource {
				resource1.Policy = c1.Policy
				resource1.Rule = c1.Rule
				resource1.Status = c1.Status
				resource1.Resource = c1.Resources[0].Name
					if v != resource1 {
						res.Result = boldRed.Sprintf("Fail")
					} else {
						res.Result =  "Pass"
						continue
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