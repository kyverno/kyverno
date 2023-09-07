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
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
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
	"sigs.k8s.io/yaml"
)

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

// allow os.exit to be overwritten during unit tests
func Command() *cobra.Command {
	var cmd *cobra.Command
	var removeColor, detailedResults, table bool
	applyCommandConfig := &ApplyCommandConfig{}
	cmd = &cobra.Command{
		Use:     "apply",
		Short:   command.FormatDescription(true, websiteUrl, false, description...),
		Long:    command.FormatDescription(false, websiteUrl, false, description...),
		Example: command.FormatExamples(examples...),
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
			err = exit(rc, applyCommandConfig.warnExitCode, applyCommandConfig.warnNoPassed)
			if err != nil {
				return err
			}
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

func (c *ApplyCommandConfig) applyCommandHelper() (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
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
	var userInfo *v1beta1.RequestInfo
	if c.UserInfoPath != "" {
		userInfo, err = userinfo.Load(nil, c.UserInfoPath, "")
		if err != nil {
			return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("Error: failed to load request info", err)
		}
	}
	variables, err := variables.New(nil, "", c.ValuesFile, nil, c.Variables...)
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
	resources, err := c.loadResources(policies, validatingAdmissionPolicies, dClient)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	rc, uu, skipInvalidPolicies, er, err = c.applyPolicytoResource(variables, policies, validatingAdmissionPolicies, resources, openApiManager, skipInvalidPolicies, dClient, userInfo, mutateLogPathIsDir)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	rc, uu, skipInvalidPolicies, er, err = c.applyValidatingAdmissionPolicytoResource(variables, validatingAdmissionPolicies, resources, rc, dClient, skipInvalidPolicies, er)
	if err != nil {
		return rc, uu, skipInvalidPolicies, er, err
	}
	return rc, resources, skipInvalidPolicies, er, nil
}

func (c *ApplyCommandConfig) getMutateLogPathIsDir(skipInvalidPolicies SkippedInvalidPolicies) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, bool) {
	mutateLogPathIsDir, err := checkMutateLogPath(c.MutateLogPath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to create file/folder", err), false
		}
		return nil, nil, skipInvalidPolicies, nil, err, false
	}
	return nil, nil, skipInvalidPolicies, nil, err, mutateLogPathIsDir
}

func (c *ApplyCommandConfig) applyValidatingAdmissionPolicytoResource(
	variables *variables.Variables,
	validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy,
	resources []*unstructured.Unstructured,
	rc *processor.ResultCounts,
	dClient dclient.Interface,
	skipInvalidPolicies SkippedInvalidPolicies,
	responses []engineapi.EngineResponse,
) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	for _, resource := range resources {
		for _, policy := range validatingAdmissionPolicies {
			processor := processor.ValidatingAdmissionPolicyProcessor{
				ValidatingAdmissionPolicy: policy,
				Resource:                  resource,
				PolicyReport:              c.PolicyReport,
				Rc:                        rc,
			}
			ers, err := processor.ApplyPolicyOnResource()
			if err != nil {
				return rc, resources, skipInvalidPolicies, responses, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			responses = append(responses, ers...)
		}
	}
	return rc, resources, skipInvalidPolicies, responses, nil
}

func (c *ApplyCommandConfig) applyPolicytoResource(
	vars *variables.Variables,
	policies []kyvernov1.PolicyInterface,
	validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy,
	resources []*unstructured.Unstructured,
	openApiManager openapi.Manager,
	skipInvalidPolicies SkippedInvalidPolicies,
	dClient dclient.Interface,
	userInfo *v1beta1.RequestInfo,
	mutateLogPathIsDir bool,
) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	if vars != nil {
		vars.SetInStore()
	}
	if !c.Stdin {
		var policyRulesCount int
		for _, policy := range policies {
			policyRulesCount += len(autogen.ComputeRules(policy))
		}
		policyRulesCount += len(validatingAdmissionPolicies)
		fmt.Printf("\nApplying %d policy rule(s) to %d resource(s)...\n", policyRulesCount, len(resources))
	}

	var rc processor.ResultCounts
	var responses []engineapi.EngineResponse
	for _, resource := range resources {
		for _, pol := range policies {
			_, err := policyvalidation.Validate(pol, nil, nil, true, openApiManager, config.KyvernoUserName(config.KyvernoServiceAccountName()))
			if err != nil {
				log.Log.Error(err, "policy validation error")
				if strings.HasPrefix(err.Error(), "variable 'element.name'") {
					skipInvalidPolicies.invalid = append(skipInvalidPolicies.invalid, pol.GetName())
				} else {
					skipInvalidPolicies.skipped = append(skipInvalidPolicies.skipped, pol.GetName())
				}

				continue
			}
			matches, err := policy.ExtractVariables(pol)
			if err != nil {
				log.Log.Error(err, "skipping invalid policy", "name", pol.GetName())
				continue
			}
			if !vars.HasVariables() && variables.NeedsVariables(matches...) {
				// check policy in variable file
				if !vars.HasPolicyVariables(pol.GetName()) {
					skipInvalidPolicies.skipped = append(skipInvalidPolicies.skipped, pol.GetName())
					continue
				}
			}
			kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(pol, vars.Subresources(), dClient)
			resourceValues, err := vars.CheckVariableForPolicy(pol.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied, matches...)
			if err != nil {
				return &rc, resources, skipInvalidPolicies, responses, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", pol.GetName(), resource.GetName()), err)
			}
			processor := processor.PolicyProcessor{
				Policy:               pol,
				Resource:             resource,
				MutateLogPath:        c.MutateLogPath,
				MutateLogPathIsDir:   mutateLogPathIsDir,
				Variables:            resourceValues,
				UserInfo:             userInfo,
				PolicyReport:         c.PolicyReport,
				NamespaceSelectorMap: vars.NamespaceSelectors(),
				Stdin:                c.Stdin,
				Rc:                   &rc,
				PrintPatchResource:   true,
				Client:               dClient,
				AuditWarn:            c.AuditWarn,
				Subresources:         vars.Subresources(),
			}
			ers, err := processor.ApplyPolicyOnResource()
			if err != nil {
				return &rc, resources, skipInvalidPolicies, responses, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", pol.GetName(), resource.GetName()).Error(), err)
			}
			responses = append(responses, processSkipEngineResponses(ers)...)
		}
	}
	return &rc, resources, skipInvalidPolicies, responses, nil
}

func (c *ApplyCommandConfig) loadResources(policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, dClient dclient.Interface) ([]*unstructured.Unstructured, error) {
	resources, err := common.GetResourceAccordingToResourcePath(nil, c.ResourcePaths, c.Cluster, policies, validatingAdmissionPolicies, dClient, c.Namespace, c.PolicyReport, "")
	if err != nil {
		return resources, fmt.Errorf("Error: failed to load resources\nCause: %s\n", err)
	}
	return resources, nil
}

func (c *ApplyCommandConfig) loadPolicies(skipInvalidPolicies SkippedInvalidPolicies) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, []kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy) {
	// load policies
	var policies []kyvernov1.PolicyInterface
	var validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy

	for _, path := range c.PolicyPaths {
		isGit := source.IsGit(path)

		if isGit {
			gitSourceURL, err := url.Parse(path)
			if err != nil {
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("Error: failed to load policies", err), nil, nil
			}

			pathElems := strings.Split(gitSourceURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				err := fmt.Errorf("invalid URL path %s - expected https://<any_git_source_domain>/:owner/:repository/:branch (without --git-branch flag) OR https://<any_git_source_domain>/:owner/:repository/:directory (with --git-branch flag)", gitSourceURL.Path)
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("Error: failed to parse URL", err), nil, nil
			}
			gitSourceURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
			repoURL := gitSourceURL.String()
			var gitPathToYamls string
			c.GitBranch, gitPathToYamls = common.GetGitBranchOrPolicyPaths(c.GitBranch, repoURL, path)
			fs := memfs.New()
			if _, err := gitutils.Clone(repoURL, fs, c.GitBranch); err != nil {
				log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", err)
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("Error: failed to clone repository", err), nil, nil
			}
			policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("failed to list YAMLs in repository", err), nil, nil
			}
			for _, policyYaml := range policyYamls {
				policiesFromFile, admissionPoliciesFromFile, err := policy.Load(fs, "", policyYaml)
				if err != nil {
					continue
				}
				policies = append(policies, policiesFromFile...)
				validatingAdmissionPolicies = append(validatingAdmissionPolicies, admissionPoliciesFromFile...)
			}
		} else {
			policiesFromFile, admissionPoliciesFromFile, err := policy.Load(nil, "", path)
			if err != nil {
				return nil, nil, skipInvalidPolicies, nil, sanitizederror.NewWithError("Error: failed to load policies", err), nil, nil
			}
			policies = append(policies, policiesFromFile...)
			validatingAdmissionPolicies = append(validatingAdmissionPolicies, admissionPoliciesFromFile...)
		}
	}

	return nil, nil, skipInvalidPolicies, nil, nil, policies, validatingAdmissionPolicies
}

func (c *ApplyCommandConfig) initStoreAndClusterClient(skipInvalidPolicies SkippedInvalidPolicies) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, dclient.Interface) {
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

func (c *ApplyCommandConfig) cleanPreviousContent(mutateLogPathIsDir bool, skipInvalidPolicies SkippedInvalidPolicies) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
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

func (c *ApplyCommandConfig) checkArguments() (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
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
	clustered, namespaced := report.ComputePolicyReports(auditWarn, engineResponses...)
	if len(clustered) > 0 || len(namespaced) > 0 {
		fmt.Println(divider)
		fmt.Println("POLICY REPORT:")
		fmt.Println(divider)
		report := report.MergeClusterReports(clustered, namespaced)
		yamlReport, _ := yaml.Marshal(report)
		fmt.Println(string(yamlReport))
	} else {
		fmt.Println(divider)
		fmt.Println("POLICY REPORT: skip generating policy report (no validate policy found/resource skipped)")
	}
}

func printViolations(rc *processor.ResultCounts) {
	fmt.Printf("\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n", rc.Pass(), rc.Fail(), rc.Warn(), rc.Error(), rc.Skip())
}

func exit(rc *processor.ResultCounts, warnExitCode int, warnNoPassed bool) error {
	if rc.Fail() > 0 || rc.Error() > 0 {
		return fmt.Errorf("exit as fail or error count > 0")
	} else if rc.Warn() > 0 && warnExitCode != 0 {
		return fmt.Errorf("exit as warnExitCode is %d", warnExitCode)
	} else if rc.Pass() == 0 && warnNoPassed {
		return fmt.Errorf("exit as warnExitCode is %d", warnExitCode)
	}
	return nil
}

func processSkipEngineResponses(responses []engineapi.EngineResponse) []engineapi.EngineResponse {
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
