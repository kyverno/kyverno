package apply

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/values"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/spf13/cobra"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// load res
// load pol
// apply
// show res
const divider = "----------------------------------------------------------------------"

type SkippedInvalidPolicies struct {
	skipped []string
	invalid []string
}

type ApplyCommandConfig struct {
	KubeConfig     string
	Context        string
	Namespace      string
	MutateLogPath  string
	Variables      []string
	ValuesFile     string
	UserInfoPath   string
	Cluster        bool
	PolicyReport   bool
	Stdin          bool
	RegistryAccess bool
	AuditWarn      bool
	ResourcePaths  []string
	PolicyPaths    []string
	GitBranch      string
	warnExitCode   int
	warnNoPassed   bool
}

var (
	applyHelp = `
To apply on a resource:
        kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2

To apply on a folder of resources:
        kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resources/

To apply on a cluster:
        kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster

To apply policies from a gitSourceURL on a cluster:
    Example: Taking github.com as a gitSourceURL here. Some other standards  gitSourceURL are: gitlab.com , bitbucket.org , etc.
        kyverno apply https://github.com/kyverno/policies/openshift/ --git-branch main --cluster

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

More info: https://kyverno.io/docs/kyverno-cli/
`

	// allow os.exit to be overwritten during unit tests
	osExit = os.Exit
)

func Command() *cobra.Command {
	var cmd *cobra.Command
	var removeColor, detailedResults, table bool
	applyCommandConfig := &ApplyCommandConfig{}
	cmd = &cobra.Command{
		Use:     "apply",
		Short:   "Applies policies on resources.",
		Example: applyHelp,
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			color.InitColors(removeColor)
			defer func() {
				if err != nil {
					if !sanitizederror.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")
					}
				}
			}()
			applyCommandConfig.PolicyPaths = policyPaths
			rc, _, skipInvalidPolicies, responses, err := applyCommandConfig.applyCommandHelper()
			if err != nil {
				return err
			}
			printSkippedAndInvalidPolicies(skipInvalidPolicies)
			if applyCommandConfig.PolicyReport {
				printReport(responses, applyCommandConfig.AuditWarn)
			} else if table {
				printTable(detailedResults, applyCommandConfig.AuditWarn, responses...)
			} else {
				printViolations(rc)
			}
			exit(rc, applyCommandConfig.warnExitCode, applyCommandConfig.warnNoPassed)
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&applyCommandConfig.ResourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().BoolVarP(&applyCommandConfig.Cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&applyCommandConfig.MutateLogPath, "output", "o", "", "Prints the mutated resources in provided file/directory")
	// currently `set` flag supports variable for single policy applied on single resource
	cmd.Flags().StringVarP(&applyCommandConfig.UserInfoPath, "userinfo", "u", "", "Admission Info including Roles, Cluster Roles and Subjects")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Variables, "set", "s", nil, "Variables that are required")
	cmd.Flags().StringVarP(&applyCommandConfig.ValuesFile, "values-file", "f", "", "File containing values for policy variables")
	cmd.Flags().BoolVarP(&applyCommandConfig.PolicyReport, "policy-report", "p", false, "Generates policy report when passed (default policyviolation)")
	cmd.Flags().StringVarP(&applyCommandConfig.Namespace, "namespace", "n", "", "Optional Policy parameter passed with cluster flag")
	cmd.Flags().BoolVarP(&applyCommandConfig.Stdin, "stdin", "i", false, "Optional mutate policy parameter to pipe directly through to kubectl")
	cmd.Flags().BoolVar(&applyCommandConfig.RegistryAccess, "registry", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().StringVar(&applyCommandConfig.KubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&applyCommandConfig.Context, "context", "", "The name of the kubeconfig context to use")
	cmd.Flags().StringVarP(&applyCommandConfig.GitBranch, "git-branch", "b", "", "test git repository branch")
	cmd.Flags().BoolVar(&applyCommandConfig.AuditWarn, "audit-warn", false, "If set to true, will flag audit policies as warnings instead of failures")
	cmd.Flags().IntVar(&applyCommandConfig.warnExitCode, "warn-exit-code", 0, "Set the exit code for warnings; if failures or errors are found, will exit 1")
	cmd.Flags().BoolVar(&applyCommandConfig.warnNoPassed, "warn-no-pass", false, "Specify if warning exit code should be raised if no objects satisfied a policy; can be used together with --warn-exit-code flag")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	cmd.Flags().BoolVar(&detailedResults, "detailed-results", false, "If set to true, display detailed results")
	cmd.Flags().BoolVarP(&table, "table", "t", false, "Show results in table format")
	return cmd
}

func (c *ApplyCommandConfig) applyCommandHelper() (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	rc, uu, skipInvalidPolicies, er, err := c.checkArguments()
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	rc, uu, skipInvalidPolicies, er, err, mutateLogPathIsDir := c.getMutateLogPathIsDir(skipInvalidPolicies)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	rc, uu, skipInvalidPolicies, er, err = c.cleanPreviousContent(mutateLogPathIsDir, skipInvalidPolicies)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	var userInfo v1beta1.RequestInfo
	if c.UserInfoPath != "" {
		userInfo, err = common.GetUserInfoFromPath(nil, c.UserInfoPath, false, "")
		if err != nil {
			fmt.Printf("Error: failed to load request info\nCause: %s\n", err)
			osExit(1)
		}
	}
	variables, globalValMap, valuesMap, namespaceSelectorMap, subresources, err := common.GetVariable(c.Variables, c.ValuesFile, nil, false, "")
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return nil, nil, skipInvalidPolicies, nil, err
	}
	openApiManager, err := openapi.NewManager(log.Log)
	if err != nil {
		return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to initialize openAPIController", err)
	}
	rc, uu, skipInvalidPolicies, er, err, dClient := c.initStoreAndClusterClient(skipInvalidPolicies)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	rc, uu, skipInvalidPolicies, er, err, policies, validatingAdmissionPolicies := c.loadPolicies(skipInvalidPolicies)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	resources := c.loadResources(policies, validatingAdmissionPolicies, dClient)
	rc, uu, skipInvalidPolicies, er, err = c.applyPolicytoResource(variables, policies, validatingAdmissionPolicies, resources, openApiManager, skipInvalidPolicies, valuesMap, dClient, subresources, globalValMap, userInfo, mutateLogPathIsDir, namespaceSelectorMap)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	rc, uu, skipInvalidPolicies, er, err = c.applyValidatingAdmissionPolicytoResource(validatingAdmissionPolicies, resources, rc, dClient, subresources, skipInvalidPolicies, er)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	return rc, resources, skipInvalidPolicies, er, nil
}

func (c *ApplyCommandConfig) getMutateLogPathIsDir(skipInvalidPolicies SkippedInvalidPolicies) (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, bool) {
	mutateLogPathIsDir, err := checkMutateLogPath(c.MutateLogPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to create file/folder", err), false
		}
		return nil, nil, skipInvalidPolicies, nil, err, false
	}
	return nil, nil, skipInvalidPolicies, nil, err, mutateLogPathIsDir
}

func (c *ApplyCommandConfig) applyValidatingAdmissionPolicytoResource(validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, resources []*unstructured.Unstructured, rc *common.ResultCounts, dClient dclient.Interface, subresources []values.Subresource, skipInvalidPolicies SkippedInvalidPolicies, responses []engineapi.EngineResponse) (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	validatingAdmissionPolicy := common.ValidatingAdmissionPolicies{}
	for _, resource := range resources {
		for _, policy := range validatingAdmissionPolicies {
			applyPolicyConfig := common.ApplyPolicyConfig{
				ValidatingAdmissionPolicy: policy,
				Resource:                  resource,
				PolicyReport:              c.PolicyReport,
				Rc:                        rc,
				Client:                    dClient,
				AuditWarn:                 c.AuditWarn,
				Subresources:              subresources,
			}
			ers, err := validatingAdmissionPolicy.ApplyPolicyOnResource(applyPolicyConfig)
			if err != nil {
				return rc, resources, skipInvalidPolicies, responses, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			responses = append(responses, ers...)
		}
	}
	return rc, resources, skipInvalidPolicies, responses, nil
}

func (c *ApplyCommandConfig) applyPolicytoResource(variables map[string]string, policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, resources []*unstructured.Unstructured, openApiManager openapi.Manager, skipInvalidPolicies SkippedInvalidPolicies, valuesMap map[string]map[string]values.Resource, dClient dclient.Interface, subresources []values.Subresource, globalValMap map[string]string, userInfo v1beta1.RequestInfo, mutateLogPathIsDir bool, namespaceSelectorMap map[string]map[string]string) (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	if len(variables) != 0 {
		variables = common.SetInStoreContext(policies, variables)
	}

	if !c.Stdin {
		var policyRulesCount int
		for _, policy := range policies {
			policyRulesCount += len(autogen.ComputeRules(policy))
		}
		policyRulesCount += len(validatingAdmissionPolicies)
		fmt.Printf("\nApplying %d policy rule(s) to %d resource(s)...\n", policyRulesCount, len(resources))
	}

	var rc common.ResultCounts
	var responses []engineapi.EngineResponse
	for _, resource := range resources {
		for _, policy := range policies {

			_, err := policyvalidation.Validate(policy, nil, nil, true, openApiManager, config.KyvernoUserName(config.KyvernoServiceAccountName()))
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
			kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, subresources, dClient)
			thisPolicyResourceValues, err := common.CheckVariableForPolicy(valuesMap, globalValMap, policy.GetName(), resource.GetName(), resource.GetKind(), variables, kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				return &rc, resources, skipInvalidPolicies, responses, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy.GetName(), resource.GetName()), err)
			}
			applyPolicyConfig := common.ApplyPolicyConfig{
				Policy:               policy,
				Resource:             resource,
				MutateLogPath:        c.MutateLogPath,
				MutateLogPathIsDir:   mutateLogPathIsDir,
				Variables:            thisPolicyResourceValues,
				UserInfo:             userInfo,
				PolicyReport:         c.PolicyReport,
				NamespaceSelectorMap: namespaceSelectorMap,
				Stdin:                c.Stdin,
				Rc:                   &rc,
				PrintPatchResource:   true,
				Client:               dClient,
				AuditWarn:            c.AuditWarn,
				Subresources:         subresources,
			}
			ers, err := common.ApplyPolicyOnResource(applyPolicyConfig)
			if err != nil {
				return &rc, resources, skipInvalidPolicies, responses, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			responses = append(responses, processSkipEngineResponses(ers, applyPolicyConfig)...)
		}
	}
	return &rc, resources, skipInvalidPolicies, responses, nil
}

func (c *ApplyCommandConfig) loadResources(policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, dClient dclient.Interface) []*unstructured.Unstructured {
	resources, err := common.GetResourceAccordingToResourcePath(nil, c.ResourcePaths, c.Cluster, policies, validatingAdmissionPolicies, dClient, c.Namespace, c.PolicyReport, false, "")
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		osExit(1)
	}
	return resources
}

func (c *ApplyCommandConfig) loadPolicies(skipInvalidPolicies SkippedInvalidPolicies) (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, []kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy) {
	// load policies
	fs := memfs.New()
	var policies []kyvernov1.PolicyInterface
	var validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy

	for _, policy := range c.PolicyPaths {
		policyPaths := []string{policy}
		isGit := common.IsGitSourcePath(policyPaths)

		if isGit {
			gitSourceURL, err := url.Parse(policyPaths[0])
			if err != nil {
				fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
				osExit(1)
			}

			pathElems := strings.Split(gitSourceURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				err := fmt.Errorf("invalid URL path %s - expected https://<any_git_source_domain>/:owner/:repository/:branch (without --git-branch flag) OR https://<any_git_source_domain>/:owner/:repository/:directory (with --git-branch flag)", gitSourceURL.Path)
				fmt.Printf("Error: failed to parse URL \nCause: %s\n", err)
				osExit(1)
			}
			gitSourceURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
			repoURL := gitSourceURL.String()
			var gitPathToYamls string
			c.GitBranch, gitPathToYamls = common.GetGitBranchOrPolicyPaths(c.GitBranch, repoURL, policyPaths)
			_, cloneErr := gitutils.Clone(repoURL, fs, c.GitBranch)
			if cloneErr != nil {
				fmt.Printf("Error: failed to clone repository \nCause: %s\n", cloneErr)
				log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", cloneErr)
				osExit(1)
			}
			policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to list YAMLs in repository", err), nil, nil
			}

			policyPaths = policyYamls
		}

		policiesFromFile, admissionPoliciesFromFile, err := common.GetPoliciesFromPaths(fs, policyPaths, isGit, "")
		if err != nil {
			fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
			osExit(1)
		}

		policies = append(policies, policiesFromFile...)
		validatingAdmissionPolicies = append(validatingAdmissionPolicies, admissionPoliciesFromFile...)
	}

	return nil, nil, skipInvalidPolicies, nil, nil, policies, validatingAdmissionPolicies
}

func (c *ApplyCommandConfig) initStoreAndClusterClient(skipInvalidPolicies SkippedInvalidPolicies) (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, dclient.Interface) {
	store.SetLocal(true)
	store.SetRegistryAccess(c.RegistryAccess)
	if c.Cluster {
		store.AllowApiCall(true)
	}
	var err error
	var dClient dclient.Interface
	if c.Cluster {
		restConfig, err := config.CreateClientConfigWithContext(c.KubeConfig, c.Context)
		if err != nil {
			return nil, nil, skipInvalidPolicies, nil, err, nil
		}
		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, nil, skipInvalidPolicies, nil, err, nil
		}
		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			return nil, nil, skipInvalidPolicies, nil, err, nil
		}
		dClient, err = dclient.NewClient(context.Background(), dynamicClient, kubeClient, 15*time.Minute)
		if err != nil {
			return nil, nil, skipInvalidPolicies, nil, err, nil
		}
	}
	return nil, nil, skipInvalidPolicies, nil, err, dClient
}

func (c *ApplyCommandConfig) cleanPreviousContent(mutateLogPathIsDir bool, skipInvalidPolicies SkippedInvalidPolicies) (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	// empty the previous contents of the file just in case if the file already existed before with some content(so as to perform overwrites)
	// the truncation of files for the case when mutateLogPath is dir, is handled under pkg/kyverno/apply/common.go
	if !mutateLogPathIsDir && c.MutateLogPath != "" {
		c.MutateLogPath = filepath.Clean(c.MutateLogPath)
		// Necessary for us to include the file via variable as it is part of the CLI.
		_, err := os.OpenFile(c.MutateLogPath, os.O_TRUNC|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			if !sanitizederror.IsErrorSanitized(err) {
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to truncate the existing file at "+c.MutateLogPath, err)
			}
			return nil, nil, skipInvalidPolicies, nil, err
		}
	}
	return nil, nil, skipInvalidPolicies, nil, nil
}

func (c *ApplyCommandConfig) checkArguments() (*common.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	var skipInvalidPolicies SkippedInvalidPolicies
	if c.ValuesFile != "" && c.Variables != nil {
		return nil, nil, skipInvalidPolicies, nil, sanitizederror.New("pass the values either using set flag or values_file flag")
	}
	if len(c.PolicyPaths) == 0 {
		return nil, nil, skipInvalidPolicies, nil, sanitizederror.New("require policy")
	}
	if (len(c.PolicyPaths) > 0 && c.PolicyPaths[0] == "-") && len(c.ResourcePaths) > 0 && c.ResourcePaths[0] == "-" {
		return nil, nil, skipInvalidPolicies, nil, sanitizederror.New("a stdin pipe can be used for either policies or resources, not both")
	}
	if len(c.ResourcePaths) == 0 && !c.Cluster {
		return nil, nil, skipInvalidPolicies, nil, sanitizederror.New("resource file(s) or cluster required")
	}
	return nil, nil, skipInvalidPolicies, nil, nil
}

func printSkippedAndInvalidPolicies(skipInvalidPolicies SkippedInvalidPolicies) {
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
}

func printReport(engineResponses []engineapi.EngineResponse, auditWarn bool) {
	clustered, namespaced := buildPolicyReports(auditWarn, engineResponses...)
	if len(clustered) > 0 || len(namespaced) > 0 {
		fmt.Println(divider)
		fmt.Println("POLICY REPORT:")
		fmt.Println(divider)
		report := mergeClusterReport(clustered, namespaced)
		yamlReport, _ := yaml.Marshal(report)
		fmt.Println(string(yamlReport))
	} else {
		fmt.Println(divider)
		fmt.Println("POLICY REPORT: skip generating policy report (no validate policy found/resource skipped)")
	}
}

func printViolations(rc *common.ResultCounts) {
	fmt.Printf("\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n", rc.Pass, rc.Fail, rc.Warn, rc.Error, rc.Skip)
}

func exit(rc *common.ResultCounts, warnExitCode int, warnNoPassed bool) {
	if rc.Fail > 0 || rc.Error > 0 {
		osExit(1)
	} else if rc.Warn > 0 && warnExitCode != 0 {
		osExit(warnExitCode)
	} else if rc.Pass == 0 && warnNoPassed {
		osExit(warnExitCode)
	}
}

func processSkipEngineResponses(responses []engineapi.EngineResponse, c common.ApplyPolicyConfig) []engineapi.EngineResponse {
	var processedEngineResponses []engineapi.EngineResponse
	for _, response := range responses {
		if !response.IsEmpty() {
			pol := response.Policy()
			if polType := pol.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
				return processedEngineResponses
			}

			for _, rule := range autogen.ComputeRules(pol.GetPolicy().(kyvernov1.PolicyInterface)) {
				if rule.HasValidate() || rule.HasVerifyImageChecks() || rule.HasVerifyImages() {
					ruleFoundInEngineResponse := false
					for _, valResponseRule := range response.PolicyResponse.Rules {
						if rule.Name == valResponseRule.Name() {
							ruleFoundInEngineResponse = true
						}
					}
					if !ruleFoundInEngineResponse {
						response.PolicyResponse.Rules = append(response.PolicyResponse.Rules,
							*engineapi.RuleSkip(
								rule.Name,
								engineapi.Validation,
								rule.Validation.Message,
							),
						)
					}
				}
			}
		}
		processedEngineResponses = append(processedEngineResponses, response)
	}
	return processedEngineResponses
}
