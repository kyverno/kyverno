package apply

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/openapi"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	yaml1 "sigs.k8s.io/yaml"
)

type resultCounts struct {
	pass  int
	fail  int
	warn  int
	error int
	skip  int
}

type Resource struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources"`
}

type Values struct {
	Policies []Policy `json:"policies"`
}

type SkippedPolicy struct {
	Name     string    `json:"name"`
	Rules    []v1.Rule `json:"rules"`
	Variable string    `json:"variable"`
}

var applyHelp = `
To apply on a resource:
	kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2

To apply on a cluster:
	kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster


To apply policy with variables:

	1. To apply single policy with variable on single resource use flag "set".
		Example:
		kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --set <variable1>=<value1>,<variable2>=<value2>

	2. To apply multiple policy with variable on multiple resource use flag "values_file".
		Example:
		kyverno apply /path/to/policy1.yaml /path/to/policy2.yaml --resource /path/to/resource1.yaml --resource /path/to/resource2.yaml -f /path/to/value.yaml

		Format of value.yaml:

		policies:
			- name: <policy1 name>
				resources:
				- name: <resource1 name>
					values:
					<variable1 in policy1>: <value>
					<variable2 in policy1>: <value>
				- name: <resource2 name>
					values:
					<variable1 in policy1>: <value>
					<variable2 in policy1>: <value>
			- name: <policy2 name>
				resources:
				- name: <resource1 name>
					values:
					<variable1 in policy2>: <value>
					<variable2 in policy2>: <value>
				- name: <resource2 name>
					values:
					<variable1 in policy2>: <value>
					<variable2 in policy2>: <value>

More info: https://kyverno.io/docs/kyverno-cli/
`

func Command() *cobra.Command {
	var cmd *cobra.Command
	var resourcePaths []string
	var cluster, policyReport bool
	var mutateLogPath, variablesString, valuesFile, namespace string

	cmd = &cobra.Command{
		Use:     "apply",
		Short:   "applies policies on resources",
		Example: applyHelp,
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			defer func() {
				if err != nil {
					if !sanitizederror.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")
					}
				}
			}()

			validateEngineResponses, rc, resources, skippedPolicies, err := applyCommandHelper(resourcePaths, cluster, policyReport, mutateLogPath, variablesString, valuesFile, namespace, policyPaths)
			if err != nil {
				return err
			}

			printReportOrViolation(policyReport, validateEngineResponses, rc, resourcePaths, len(resources), skippedPolicies)
			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&resourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().BoolVarP(&cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&mutateLogPath, "output", "o", "", "Prints the mutated resources in provided file/directory")
	cmd.Flags().StringVarP(&variablesString, "set", "s", "", "Variables that are required")
	cmd.Flags().StringVarP(&valuesFile, "values-file", "f", "", "File containing values for policy variables")
	cmd.Flags().BoolVarP(&policyReport, "policy-report", "", false, "Generates policy report when passed (default policyviolation r")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Optional Policy parameter passed with cluster flag")
	return cmd
}

func applyCommandHelper(resourcePaths []string, cluster bool, policyReport bool, mutateLogPath string,
	variablesString string, valuesFile string, namespace string, policyPaths []string) (validateEngineResponses []*response.EngineResponse, rc *resultCounts, resources []*unstructured.Unstructured, skippedPolicies []SkippedPolicy, err error) {

	kubernetesConfig := genericclioptions.NewConfigFlags(true)

	if valuesFile != "" && variablesString != "" {
		return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("pass the values either using set flag or values_file flag", err)
	}

	variables, valuesMap, err := getVariable(variablesString, valuesFile)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return validateEngineResponses, rc, resources, skippedPolicies, err
	}

	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("failed to initialize openAPIController", err)
	}

	var dClient *client.Client
	if cluster {
		restConfig, err := kubernetesConfig.ToRESTConfig()
		if err != nil {
			return validateEngineResponses, rc, resources, skippedPolicies, err
		}
		dClient, err = client.NewClient(restConfig, 15*time.Minute, make(chan struct{}), log.Log)
		if err != nil {
			return validateEngineResponses, rc, resources, skippedPolicies, err
		}
	}

	if len(policyPaths) == 0 {
		return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError(fmt.Sprintf("require policy"), err)
	}

	if (len(policyPaths) > 0 && policyPaths[0] == "-") && len(resourcePaths) > 0 && resourcePaths[0] == "-" {
		return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("a stdin pipe can be used for either policies or resources, not both", err)
	}

	policies, err := getPoliciesFromPaths(policyPaths)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	if len(resourcePaths) == 0 && !cluster {
		return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError(fmt.Sprintf("resource file(s) or cluster required"), err)
	}

	mutateLogPathIsDir, err := checkMutateLogPath(mutateLogPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("failed to create file/folder", err)
		}
		return validateEngineResponses, rc, resources, skippedPolicies, err
	}

	mutatedPolicies, err := mutatePolices(policies)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("failed to mutate policy", err)
		}
	}

	resources, err = getResourceAccordingToResourcePath(resourcePaths, cluster, mutatedPolicies, dClient, namespace, policyReport)
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

	rc = &resultCounts{}
	engineResponses := make([]*response.EngineResponse, 0)
	validateEngineResponses = make([]*response.EngineResponse, 0)
	skippedPolicies = make([]SkippedPolicy, 0)

	for _, policy := range mutatedPolicies {
		err := policy2.Validate(policy, nil, true, openAPIController)
		if err != nil {
			rc.skip += len(resources)
			log.Log.V(3).Info(fmt.Sprintf("skipping policy %v as it is not valid", policy.Name), "error", err)
			continue
		}

		matches := common.PolicyHasVariables(*policy)
		variable := removeDuplicatevariables(matches)

		if len(matches) > 0 && variablesString == "" && valuesFile == "" {
			rc.skip++
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
			// get values from file for this policy resource combination
			thisPolicyResourceValues := make(map[string]string)
			if len(valuesMap[policy.GetName()]) != 0 && !reflect.DeepEqual(valuesMap[policy.GetName()][resource.GetName()], Resource{}) {
				thisPolicyResourceValues = valuesMap[policy.GetName()][resource.GetName()].Values
			}

			for k, v := range variables {
				thisPolicyResourceValues[k] = v
			}

			if len(common.PolicyHasVariables(*policy)) > 0 && len(thisPolicyResourceValues) == 0 {
				return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError(fmt.Sprintf("policy %s have variables. pass the values for the variables using set/values_file flag", policy.Name), err)
			}

			ers, validateErs, err := applyPolicyOnResource(policy, resource, mutateLogPath, mutateLogPathIsDir, thisPolicyResourceValues, rc, policyReport)
			if err != nil {
				return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
			}

			engineResponses = append(engineResponses, ers...)
			validateEngineResponses = append(validateEngineResponses, validateErs)
		}
	}

	return validateEngineResponses, rc, resources, skippedPolicies, nil
}

// getVariable - get the variables from console/file
func getVariable(variablesString, valuesFile string) (variables map[string]string, valuesMap map[string]map[string]Resource, err error) {
	if variablesString != "" {
		kvpairs := strings.Split(strings.Trim(variablesString, " "), ",")
		for _, kvpair := range kvpairs {
			kvs := strings.Split(strings.Trim(kvpair, " "), "=")
			variables[strings.Trim(kvs[0], " ")] = strings.Trim(kvs[1], " ")
		}
	}

	if valuesFile != "" {
		yamlFile, err := ioutil.ReadFile(valuesFile)
		if err != nil {
			return variables, valuesMap, sanitizederror.NewWithError("unable to read yaml", err)
		}

		valuesBytes, err := yaml.ToJSON(yamlFile)
		if err != nil {
			return variables, valuesMap, sanitizederror.NewWithError("failed to convert json", err)
		}

		values := &Values{}
		if err := json.Unmarshal(valuesBytes, values); err != nil {
			return variables, valuesMap, sanitizederror.NewWithError("failed to decode yaml", err)
		}

		for _, p := range values.Policies {
			pmap := make(map[string]Resource)
			for _, r := range p.Resources {
				pmap[r.Name] = r
			}
			valuesMap[p.Name] = pmap
		}
	}

	return variables, valuesMap, nil
}

// checkMutateLogPath - checking path for printing mutated resource (-o flag)
func checkMutateLogPath(mutateLogPath string) (mutateLogPathIsDir bool, err error) {
	if mutateLogPath != "" {
		spath := strings.Split(mutateLogPath, "/")
		sfileName := strings.Split(spath[len(spath)-1], ".")
		if sfileName[len(sfileName)-1] == "yml" || sfileName[len(sfileName)-1] == "yaml" {
			mutateLogPathIsDir = false
		} else {
			mutateLogPathIsDir = true
		}

		err := createFileOrFolder(mutateLogPath, mutateLogPathIsDir)
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return mutateLogPathIsDir, sanitizederror.NewWithError("failed to create file/folder.", err)
			}
			return mutateLogPathIsDir, err
		}
	}
	return mutateLogPathIsDir, err
}

// getPoliciesFromPaths - get policies according to the resource path
func getPoliciesFromPaths(policyPaths []string) (policies []*v1.ClusterPolicy, err error) {
	if len(policyPaths) > 0 && policyPaths[0] == "-" {
		if common.IsInputFromPipe() {
			policyStr := ""
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				policyStr = policyStr + scanner.Text() + "\n"
			}

			yamlBytes := []byte(policyStr)
			policies, err = utils.GetPolicy(yamlBytes)
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
	return
}

// getResourceAccordingToResourcePath - get resources according to the resource path
func getResourceAccordingToResourcePath(resourcePaths []string, cluster bool, policies []*v1.ClusterPolicy, dClient *client.Client, namespace string, policyReport bool) (resources []*unstructured.Unstructured, err error) {
	if len(resourcePaths) > 0 && resourcePaths[0] == "-" {
		if common.IsInputFromPipe() {
			resourceStr := ""
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				resourceStr = resourceStr + scanner.Text() + "\n"
			}

			yamlBytes := []byte(resourceStr)
			resources, err = common.GetResource(yamlBytes)
			if err != nil {
				return nil, sanitizederror.NewWithError("failed to extract the resources", err)
			}
		}
	} else if (len(resourcePaths) > 0 && resourcePaths[0] != "-") || len(resourcePaths) < 0 || cluster {
		resources, err = common.GetResources(policies, resourcePaths, dClient, cluster, namespace, policyReport)
		if err != nil {
			return resources, err
		}
	}
	return resources, err
}

// printReportOrViolation - printing policy report/violations
func printReportOrViolation(policyReport bool, validateEngineResponses []*response.EngineResponse, rc *resultCounts, resourcePaths []string, resourcesLen int, skippedPolicies []SkippedPolicy) {
	if policyReport {
		os.Setenv("POLICY-TYPE", pkgCommon.PolicyReport)
		resps := buildPolicyReports(validateEngineResponses, skippedPolicies)
		if len(resps) > 0 || resourcesLen == 0 {
			fmt.Println("----------------------------------------------------------------------\nPOLICY REPORT:\n----------------------------------------------------------------------")
			report, _ := generateCLIraw(resps)
			yamlReport, _ := yaml1.Marshal(report)
			fmt.Println(string(yamlReport))
		} else {
			fmt.Println("----------------------------------------------------------------------\nPOLICY REPORT: skip generating policy report (no validate policy found/resource skipped)")
		}
	} else {
		rcCount := rc.pass + rc.fail + rc.warn + rc.error + rc.skip
		if rcCount < len(resourcePaths) {
			rc.skip += len(resourcePaths) - rcCount
		}

		fmt.Printf("\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n",
			rc.pass, rc.fail, rc.warn, rc.error, rc.skip)

		if rc.fail > 0 || rc.error > 0 {
			os.Exit(1)
		}
	}
}

// applyPolicyOnResource - function to apply policy on resource
func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured,
	mutateLogPath string, mutateLogPathIsDir bool, variables map[string]string,
	rc *resultCounts, policyReport bool) ([]*response.EngineResponse, *response.EngineResponse, error) {

	responseError := false
	engineResponses := make([]*response.EngineResponse, 0)

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", policy.Name, "resource", resPath)

	ctx := context.NewContext()
	for key, value := range variables {
		startString := ""
		endString := ""
		for _, k := range strings.Split(key, ".") {
			startString += fmt.Sprintf(`{"%s":`, k)
			endString += `}`
		}

		midString := fmt.Sprintf(`"%s"`, value)
		finalString := startString + midString + endString
		var jsonData = []byte(finalString)
		ctx.AddJSON(jsonData)
	}

	mutateResponse := engine.Mutate(&engine.PolicyContext{Policy: *policy, NewResource: *resource, JSONContext: ctx}, nil)
	engineResponses = append(engineResponses, mutateResponse)

	if !mutateResponse.IsSuccessful() {
		fmt.Printf("Failed to apply mutate policy %s -> resource %s", policy.Name, resPath)
		for i, r := range mutateResponse.PolicyResponse.Rules {
			fmt.Printf("\n%d. %s", i+1, r.Message)
		}
		responseError = true
	} else {
		if len(mutateResponse.PolicyResponse.Rules) > 0 {
			yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
			if err != nil {
				rc.error++
			}

			if mutateLogPath == "" {
				mutatedResource := string(yamlEncodedResource)
				if len(strings.TrimSpace(mutatedResource)) > 0 {
					fmt.Printf("\nmutate policy %s applied to %s:", policy.Name, resPath)
					fmt.Printf("\n" + mutatedResource)
					fmt.Printf("\n")
				}
			} else {
				err := printMutatedOutput(mutateLogPath, mutateLogPathIsDir, string(yamlEncodedResource), resource.GetName()+"-mutated")
				if err != nil {
					return engineResponses, &response.EngineResponse{}, sanitizederror.NewWithError("failed to print mutated result", err)
				}
				fmt.Printf("\n\nMutation:\nMutation has been applied successfully. Check the files.")
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
	validateResponse := engine.Validate(policyCtx, nil)
	if !policyReport {
		if !validateResponse.IsSuccessful() {
			fmt.Printf("\npolicy %s -> resource %s failed: \n", policy.Name, resPath)
			for i, r := range validateResponse.PolicyResponse.Rules {
				if !r.Success {
					fmt.Printf("%d. %s: %s \n", i+1, r.Name, r.Message)
				}
			}

			responseError = true
		}
	}

	var policyHasGenerate bool
	for _, rule := range policy.Spec.Rules {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}

	if policyHasGenerate {
		ctx := &engine.PolicyContext{Policy: *policy, NewResource: *resource}
		generateResponse := engine.Generate(ctx)
		engineResponses = append(engineResponses, generateResponse)
		if len(generateResponse.PolicyResponse.Rules) > 0 {
			log.Log.V(3).Info("generate resource is valid", "policy", policy.Name, "resource", resPath)
		} else {
			fmt.Printf("generate policy %s resource %s is invalid \n", policy.Name, resPath)
			for i, r := range generateResponse.PolicyResponse.Rules {
				fmt.Printf("%d. %s \b", i+1, r.Message)
			}

			responseError = true
		}
	}

	if responseError == true {
		rc.fail++
	} else {
		rc.pass++
	}

	return engineResponses, validateResponse, nil
}

// mutatePolicies - function to apply mutation on policies
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

// printMutatedOutput - function to print output in provided file or directory
func printMutatedOutput(mutateLogPath string, mutateLogPathIsDir bool, yaml string, fileName string) error {
	var f *os.File
	var err error
	yaml = yaml + ("\n---\n\n")

	if !mutateLogPathIsDir {
		f, err = os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		f, err = os.OpenFile(mutateLogPath+"/"+fileName+".yaml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}

	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(yaml)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// createFileOrFolder - creating file or folder according to path provided
func createFileOrFolder(mutateLogPath string, mutateLogPathIsDir bool) error {
	mutateLogPath = filepath.Clean(mutateLogPath)
	_, err := os.Stat(mutateLogPath)

	if err != nil {
		if os.IsNotExist(err) {
			if !mutateLogPathIsDir {
				// check the folder existence, then create the file
				var folderPath string
				s := strings.Split(mutateLogPath, "/")

				if len(s) > 1 {
					folderPath = mutateLogPath[:len(mutateLogPath)-len(s[len(s)-1])-1]
					_, err := os.Stat(folderPath)
					if os.IsNotExist(err) {
						errDir := os.MkdirAll(folderPath, 0755)
						if errDir != nil {
							return sanitizederror.NewWithError(fmt.Sprintf("failed to create directory"), err)
						}
					}
				}

				file, err := os.OpenFile(mutateLogPath, os.O_RDONLY|os.O_CREATE, 0644)
				if err != nil {
					return sanitizederror.NewWithError(fmt.Sprintf("failed to create file"), err)
				}

				err = file.Close()
				if err != nil {
					return sanitizederror.NewWithError(fmt.Sprintf("failed to close file"), err)
				}

			} else {
				errDir := os.MkdirAll(mutateLogPath, 0755)
				if errDir != nil {
					return sanitizederror.NewWithError(fmt.Sprintf("failed to create directory"), err)
				}
			}

		} else {
			return sanitizederror.NewWithError(fmt.Sprintf("failed to describe file"), err)
		}
	}

	return nil
}

// removeDuplicatevariables - remove duplicate variables
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
