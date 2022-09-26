package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/openapi"
	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	yaml1 "sigs.k8s.io/yaml"
)

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

type SkippedInvalidPolicies struct {
	skipped []string
	invalid []string
}

type ApplyCommandConfig struct {
	KubeConfig      string
	Context         string
	Namespace       string
	MutateLogPath   string
	VariablesString string
	ValuesFile      string
	UserInfoPath    string
	Cluster         bool
	PolicyReport    bool
	Stdin           bool
	RegistryAccess  bool
	ResourcePaths   []string
	PolicyPaths     []string
}

var applyHelp = `

To apply on a resource:
        kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2

To apply on a folder of resources:
        kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resources/

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
				rules:
					- name: <rule1 name>
						values:
							<context variable1 in policy1 rule1>: <value>
							<context variable2 in policy1 rule1>: <value>
					- name: <rule2 name>
						values:
							<context variable1 in policy1 rule2>: <value>
							<context variable2 in policy1 rule2>: <value>
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
	applyCommandConfig := &ApplyCommandConfig{}
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
			applyCommandConfig.PolicyPaths = policyPaths
			rc, resources, skipInvalidPolicies, pvInfos, err := applyCommandConfig.applyCommandHelper()
			if err != nil {
				return err
			}

			PrintReportOrViolation(applyCommandConfig.PolicyReport, rc, applyCommandConfig.ResourcePaths, len(resources), skipInvalidPolicies, applyCommandConfig.Stdin, pvInfos)
			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&applyCommandConfig.ResourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().BoolVarP(&applyCommandConfig.Cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&applyCommandConfig.MutateLogPath, "output", "o", "", "Prints the mutated resources in provided file/directory")
	// currently `set` flag supports variable for single policy applied on single resource
	cmd.Flags().StringVarP(&applyCommandConfig.UserInfoPath, "userinfo", "u", "", "Admission Info including Roles, Cluster Roles and Subjects")
	cmd.Flags().StringVarP(&applyCommandConfig.VariablesString, "set", "s", "", "Variables that are required")
	cmd.Flags().StringVarP(&applyCommandConfig.ValuesFile, "values-file", "f", "", "File containing values for policy variables")
	cmd.Flags().BoolVarP(&applyCommandConfig.PolicyReport, "policy-report", "p", false, "Generates policy report when passed (default policyviolation)")
	cmd.Flags().StringVarP(&applyCommandConfig.Namespace, "namespace", "n", "", "Optional Policy parameter passed with cluster flag")
	cmd.Flags().BoolVarP(&applyCommandConfig.Stdin, "stdin", "i", false, "Optional mutate policy parameter to pipe directly through to kubectl")
	cmd.Flags().BoolVarP(&applyCommandConfig.RegistryAccess, "registry", "", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().StringVarP(&applyCommandConfig.KubeConfig, "kubeconfig", "", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVarP(&applyCommandConfig.Context, "context", "", "", "The name of the kubeconfig context to use")
	return cmd
}

func (c *ApplyCommandConfig) applyCommandHelper() (rc *common.ResultCounts, resources []*unstructured.Unstructured, skipInvalidPolicies SkippedInvalidPolicies, pvInfos []policyreport.Info, err error) {
	store.SetMock(true)
	store.SetRegistryAccess(c.RegistryAccess)

	fs := memfs.New()

	if c.ValuesFile != "" && c.VariablesString != "" {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("pass the values either using set flag or values_file flag", err)
	}

	variables, globalValMap, valuesMap, namespaceSelectorMap, err := common.GetVariable(c.VariablesString, c.ValuesFile, fs, false, "")
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return rc, resources, skipInvalidPolicies, pvInfos, err
	}

	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("failed to initialize openAPIController", err)
	}

	var dClient dclient.Interface
	if c.Cluster {
		restConfig, err := config.CreateClientConfigWithContext(c.KubeConfig, c.Context)
		if err != nil {
			return rc, resources, skipInvalidPolicies, pvInfos, err
		}
		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return rc, resources, skipInvalidPolicies, pvInfos, err
		}
		dClient, err = dclient.NewClient(restConfig, kubeClient, nil, 15*time.Minute, make(chan struct{}))
		if err != nil {
			return rc, resources, skipInvalidPolicies, pvInfos, err
		}
	}

	if len(c.PolicyPaths) == 0 {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("require policy", err)
	}

	if (len(c.PolicyPaths) > 0 && c.PolicyPaths[0] == "-") && len(c.ResourcePaths) > 0 && c.ResourcePaths[0] == "-" {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("a stdin pipe can be used for either policies or resources, not both", err)
	}

	policies, err := common.GetPoliciesFromPaths(fs, c.PolicyPaths, false, "")
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	if len(c.ResourcePaths) == 0 && !c.Cluster {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("resource file(s) or cluster required", err)
	}

	mutateLogPathIsDir, err := checkMutateLogPath(c.MutateLogPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("failed to create file/folder", err)
		}
		return rc, resources, skipInvalidPolicies, pvInfos, err
	}

	// empty the previous contents of the file just in case if the file already existed before with some content(so as to perform overwrites)
	// the truncation of files for the case when mutateLogPath is dir, is handled under pkg/kyverno/apply/common.go
	if !mutateLogPathIsDir && c.MutateLogPath != "" {
		c.MutateLogPath = filepath.Clean(c.MutateLogPath)
		// Necessary for us to include the file via variable as it is part of the CLI.
		_, err := os.OpenFile(c.MutateLogPath, os.O_TRUNC|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("failed to truncate the existing file at "+c.MutateLogPath, err)
			}
			return rc, resources, skipInvalidPolicies, pvInfos, err
		}
	}

	mutatedPolicies, err := common.MutatePolicies(policies)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("failed to mutate policy", err)
		}
	}

	err = common.PrintMutatedPolicy(mutatedPolicies)
	if err != nil {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("failed to marsal mutated policy", err)
	}

	resources, err = common.GetResourceAccordingToResourcePath(fs, c.ResourcePaths, c.Cluster, mutatedPolicies, dClient, c.Namespace, c.PolicyReport, false, "")
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		os.Exit(1)
	}

	if (len(resources) > 1 || len(mutatedPolicies) > 1) && c.VariablesString != "" {
		return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError("currently `set` flag supports variable for single policy applied on single resource ", nil)
	}

	// get the user info as request info from a different file
	var userInfo v1beta1.RequestInfo
	var subjectInfo store.Subject
	if c.UserInfoPath != "" {
		userInfo, subjectInfo, err = common.GetUserInfoFromPath(fs, c.UserInfoPath, false, "")
		if err != nil {
			fmt.Printf("Error: failed to load request info\nCause: %s\n", err)
			os.Exit(1)
		}
		store.SetSubjects(subjectInfo)
	}

	if c.VariablesString != "" {
		variables = common.SetInStoreContext(mutatedPolicies, variables)
	}

	var policyRulesCount, mutatedPolicyRulesCount int
	for _, policy := range policies {
		policyRulesCount += len(policy.GetSpec().Rules)
	}

	for _, policy := range mutatedPolicies {
		mutatedPolicyRulesCount += len(policy.GetSpec().Rules)
	}

	msgPolicyRules := "1 policy rule"
	if policyRulesCount > 1 {
		msgPolicyRules = fmt.Sprintf("%d policy rules", policyRulesCount)
	}

	if mutatedPolicyRulesCount > policyRulesCount {
		msgPolicyRules = fmt.Sprintf("%d policy rules", mutatedPolicyRulesCount)
	}

	msgResources := "1 resource"
	if len(resources) > 1 {
		msgResources = fmt.Sprintf("%d resources", len(resources))
	}

	if len(mutatedPolicies) > 0 && len(resources) > 0 {
		if !c.Stdin {
			if mutatedPolicyRulesCount > policyRulesCount {
				fmt.Printf("\nauto-generated pod policies\nApplying %s to %s...\n", msgPolicyRules, msgResources)
			} else {
				fmt.Printf("\nApplying %s to %s...\n", msgPolicyRules, msgResources)
			}
		}
	}

	rc = &common.ResultCounts{}
	skipInvalidPolicies.skipped = make([]string, 0)
	skipInvalidPolicies.invalid = make([]string, 0)

	for _, policy := range mutatedPolicies {
		_, err := policy2.Validate(policy, nil, true, openAPIController)
		if err != nil {
			log.Log.Error(err, "policy validation error")
			if strings.HasPrefix(err.Error(), "variable 'element.name'") {
				skipInvalidPolicies.invalid = append(skipInvalidPolicies.invalid, policy.GetName())
			} else {
				skipInvalidPolicies.skipped = append(skipInvalidPolicies.skipped, policy.GetName())
			}

			continue
		}

		matches := common.HasVariables(policy)
		variable := common.RemoveDuplicateAndObjectVariables(matches)
		if len(variable) > 0 {
			if len(variables) == 0 {
				// check policy in variable file
				if c.ValuesFile == "" || valuesMap[policy.GetName()] == nil {
					skipInvalidPolicies.skipped = append(skipInvalidPolicies.skipped, policy.GetName())
					continue
				}
			}
		}

		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy)

		for _, resource := range resources {
			thisPolicyResourceValues, err := common.CheckVariableForPolicy(valuesMap, globalValMap, policy.GetName(), resource.GetName(), resource.GetKind(), variables, kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy.GetName(), resource.GetName()), err)
			}

			_, info, err := common.ApplyPolicyOnResource(policy, resource, c.MutateLogPath, mutateLogPathIsDir, thisPolicyResourceValues, userInfo, c.PolicyReport, namespaceSelectorMap, c.Stdin, rc, true, nil)
			if err != nil {
				return rc, resources, skipInvalidPolicies, pvInfos, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			pvInfos = append(pvInfos, info)
		}
	}

	return rc, resources, skipInvalidPolicies, pvInfos, nil
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

// PrintReportOrViolation - printing policy report/violations
func PrintReportOrViolation(policyReport bool, rc *common.ResultCounts, resourcePaths []string, resourcesLen int, skipInvalidPolicies SkippedInvalidPolicies, stdin bool, pvInfos []policyreport.Info) {
	divider := "----------------------------------------------------------------------"

	if len(skipInvalidPolicies.skipped) > 0 {
		fmt.Println(divider)
		fmt.Println("Policies Skipped (as required variables are not provided by the user):")
		for i, policyName := range skipInvalidPolicies.skipped {
			fmt.Printf("%d. %s\n", i+1, policyName)
		}
		fmt.Println(divider)
	}
	if len(skipInvalidPolicies.invalid) > 0 {
		fmt.Println(divider)
		fmt.Println("Invalid Policies:")
		for i, policyName := range skipInvalidPolicies.invalid {
			fmt.Printf("%d. %s\n", i+1, policyName)
		}
		fmt.Println(divider)
	}

	if policyReport {
		resps := buildPolicyReports(pvInfos)
		if len(resps) > 0 || resourcesLen == 0 {
			fmt.Println(divider)
			fmt.Println("POLICY REPORT:")
			fmt.Println(divider)
			report, _ := generateCLIRaw(resps)
			yamlReport, _ := yaml1.Marshal(report)
			fmt.Println(string(yamlReport))
		} else {
			fmt.Println(divider)
			fmt.Println("POLICY REPORT: skip generating policy report (no validate policy found/resource skipped)")
		}
	} else {
		if !stdin {
			fmt.Printf("\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n",
				rc.Pass, rc.Fail, rc.Warn, rc.Error, rc.Skip)
		}
	}

	if rc.Fail > 0 || rc.Error > 0 {
		os.Exit(1)
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
						errDir := os.MkdirAll(folderPath, 0o750)
						if errDir != nil {
							return sanitizederror.NewWithError("failed to create directory", err)
						}
					}
				}

				mutateLogPath = filepath.Clean(mutateLogPath)
				// Necessary for us to create the file via variable as it is part of the CLI.
				file, err := os.OpenFile(mutateLogPath, os.O_RDONLY|os.O_CREATE, 0o600) // #nosec G304
				if err != nil {
					return sanitizederror.NewWithError("failed to create file", err)
				}

				err = file.Close()
				if err != nil {
					return sanitizederror.NewWithError("failed to close file", err)
				}
			} else {
				errDir := os.MkdirAll(mutateLogPath, 0o750)
				if errDir != nil {
					return sanitizederror.NewWithError("failed to create directory", err)
				}
			}
		} else {
			return sanitizederror.NewWithError("failed to describe file", err)
		}
	}

	return nil
}
