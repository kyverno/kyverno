package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/kyverno/store"
	"github.com/kyverno/kyverno/pkg/openapi"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		namespaceSelector:
			- name: <namespace1 name>
			labels:
				<label key>: <label value>
			- name: <namespace2 name>
			labels:
				<label key>: <label value>

More info: https://kyverno.io/docs/kyverno-cli/
`

func Command() *cobra.Command {
	var cmd *cobra.Command
	var resourcePaths []string
	var cluster, policyReport, stdin bool
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

			validateEngineResponses, rc, resources, skippedPolicies, err := applyCommandHelper(resourcePaths, cluster, policyReport, mutateLogPath, variablesString, valuesFile, namespace, policyPaths, stdin)
			if err != nil {
				return err
			}

			printReportOrViolation(policyReport, validateEngineResponses, rc, resourcePaths, len(resources), skippedPolicies, stdin)
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
	cmd.Flags().BoolVarP(&stdin, "stdin", "i", false, "Optional mutate policy parameter to pipe directly through to kubectl")
	return cmd
}

func applyCommandHelper(resourcePaths []string, cluster bool, policyReport bool, mutateLogPath string,
	variablesString string, valuesFile string, namespace string, policyPaths []string, stdin bool) (validateEngineResponses []*response.EngineResponse, rc *resultCounts, resources []*unstructured.Unstructured, skippedPolicies []SkippedPolicy, err error) {

	store.SetMock(true)
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	fs := memfs.New()

	if valuesFile != "" && variablesString != "" {
		return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("pass the values either using set flag or values_file flag", err)
	}

	variables, valuesMap, namespaceSelectorMap, err := common.GetVariable(variablesString, valuesFile, fs, false, "")
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

	policies, err := common.GetPoliciesFromPaths(fs, policyPaths, false, "")
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

	// empty the previous contents of the file just in case if the file already existed before with some content(so as to perform overwrites)
	// the truncation of files for the case when mutateLogPath is dir, is handled under pkg/kyverno/apply/common.go
	if !mutateLogPathIsDir && mutateLogPath != "" {
		_, err := os.OpenFile(mutateLogPath, os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("failed to truncate the existing file at "+mutateLogPath, err)
			}
			return validateEngineResponses, rc, resources, skippedPolicies, err
		}
	}

	mutatedPolicies, err := common.MutatePolices(policies)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError("failed to mutate policy", err)
		}
	}

	resources, err = common.GetResourceAccordingToResourcePath(fs, resourcePaths, cluster, mutatedPolicies, dClient, namespace, policyReport, false, "")
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
		if !stdin {
			fmt.Printf("\napplying %s to %s... \n", msgPolicies, msgResources)
		}
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
		variable := common.RemoveDuplicateVariables(matches)

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

			ers, validateErs, responseError, rcErs, err := common.ApplyPolicyOnResource(policy, resource, mutateLogPath, mutateLogPathIsDir, thisPolicyResourceValues, policyReport, namespaceSelectorMap, stdin)
			if err != nil {
				return validateEngineResponses, rc, resources, skippedPolicies, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
			}
			if responseError == true {
				rc.fail++
			} else {
				rc.pass++
			}
			if rcErs == true {
				rc.error++
			}
			engineResponses = append(engineResponses, ers...)
			validateEngineResponses = append(validateEngineResponses, validateErs)
		}
	}

	return validateEngineResponses, rc, resources, skippedPolicies, nil
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

// printReportOrViolation - printing policy report/violations
func printReportOrViolation(policyReport bool, validateEngineResponses []*response.EngineResponse, rc *resultCounts, resourcePaths []string, resourcesLen int, skippedPolicies []SkippedPolicy, stdin bool) {
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
		if !stdin {
			fmt.Printf("\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n",
				rc.pass, rc.fail, rc.warn, rc.error, rc.skip)
		}

		if rc.fail > 0 || rc.error > 0 {
			os.Exit(1)
		}
	}
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
