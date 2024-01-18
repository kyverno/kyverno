package apply

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
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

func Command() *cobra.Command {
	var removeColor, detailedResults, table bool
	applyCommandConfig := &ApplyCommandConfig{}
	cmd := &cobra.Command{
		Use:          "apply",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			out := cmd.OutOrStdout()
			color.InitColors(removeColor)
			applyCommandConfig.PolicyPaths = args
			rc, _, skipInvalidPolicies, responses, err := applyCommandConfig.applyCommandHelper(out)
			if err != nil {
				return err
			}
			printSkippedAndInvalidPolicies(out, skipInvalidPolicies)
			if applyCommandConfig.PolicyReport {
				printReport(out, responses, applyCommandConfig.AuditWarn)
			} else if table {
				printTable(out, detailedResults, applyCommandConfig.AuditWarn, responses...)
			} else {
				printViolations(out, rc)
			}
			return exit(rc, applyCommandConfig.warnExitCode, applyCommandConfig.warnNoPassed)
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

func (c *ApplyCommandConfig) applyCommandHelper(out io.Writer) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	rc, resources1, skipInvalidPolicies, responses1, err := c.checkArguments()
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	rc, resources1, skipInvalidPolicies, responses1, err, mutateLogPathIsDir := c.getMutateLogPathIsDir(skipInvalidPolicies)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	rc, resources1, skipInvalidPolicies, responses1, err = c.cleanPreviousContent(mutateLogPathIsDir, skipInvalidPolicies)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	var userInfo *v1beta1.RequestInfo
	if c.UserInfoPath != "" {
		info, err := userinfo.Load(nil, c.UserInfoPath, "")
		if err != nil {
			return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("failed to load request info (%w)", err)
		}
		deprecations.CheckUserInfo(out, c.UserInfoPath, info)
		userInfo = &info.RequestInfo
	}
	variables, err := variables.New(out, nil, "", c.ValuesFile, nil, c.Variables...)
	if err != nil {
		return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("failed to decode yaml (%w)", err)
	}
	rc, resources1, skipInvalidPolicies, responses1, err, dClient := c.initStoreAndClusterClient(skipInvalidPolicies)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	rc, resources1, skipInvalidPolicies, responses1, policies, validatingAdmissionPolicies, err := c.loadPolicies(skipInvalidPolicies)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	resources, err := c.loadResources(out, policies, validatingAdmissionPolicies, dClient)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	if !c.Stdin {
		var policyRulesCount int
		for _, policy := range policies {
			policyRulesCount += len(autogen.ComputeRules(policy))
		}
		policyRulesCount += len(validatingAdmissionPolicies)
		fmt.Fprintf(out, "\nApplying %d policy rule(s) to %d resource(s)...\n", policyRulesCount, len(resources))
	}

	var regOpts []registryclient.Option
	if c.RegistryAccess {
		regOpts = append(regOpts, registryclient.WithLocalKeychain())
	}

	rclient := registryclient.NewOrDie(regOpts...)

	rc, resources1, responses1, err = c.applyPolicytoResource(
		out,
		variables,
		policies,
		resources,
		&skipInvalidPolicies,
		dClient,
		userInfo,
		mutateLogPathIsDir,
		rclient,
	)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	responses2, err := c.applyValidatingAdmissionPolicytoResource(variables, validatingAdmissionPolicies, resources1, rc, dClient, &skipInvalidPolicies)
	if err != nil {
		return rc, resources1, skipInvalidPolicies, responses1, err
	}
	var responses []engineapi.EngineResponse
	responses = append(responses, responses1...)
	responses = append(responses, responses2...)
	return rc, resources1, skipInvalidPolicies, responses, nil
}

func (c *ApplyCommandConfig) getMutateLogPathIsDir(skipInvalidPolicies SkippedInvalidPolicies) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error, bool) {
	mutateLogPathIsDir, err := checkMutateLogPath(c.MutateLogPath)
	if err != nil {
		return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("failed to create file/folder (%w)", err), false
	}
	return nil, nil, skipInvalidPolicies, nil, err, mutateLogPathIsDir
}

func (c *ApplyCommandConfig) applyValidatingAdmissionPolicytoResource(
	variables *variables.Variables,
	validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy,
	resources []*unstructured.Unstructured,
	rc *processor.ResultCounts,
	dClient dclient.Interface,
	skipInvalidPolicies *SkippedInvalidPolicies,
) ([]engineapi.EngineResponse, error) {
	var responses []engineapi.EngineResponse
	for _, resource := range resources {
		processor := processor.ValidatingAdmissionPolicyProcessor{
			Policies:     validatingAdmissionPolicies,
			Resource:     resource,
			PolicyReport: c.PolicyReport,
			Rc:           rc,
		}
		ers, err := processor.ApplyPolicyOnResource()
		if err != nil {
			return responses, fmt.Errorf("failed to apply policies on resource %s (%w)", resource.GetName(), err)
		}
		responses = append(responses, ers...)
	}
	return responses, nil
}

func (c *ApplyCommandConfig) applyPolicytoResource(
	out io.Writer,
	vars *variables.Variables,
	policies []kyvernov1.PolicyInterface,
	resources []*unstructured.Unstructured,
	skipInvalidPolicies *SkippedInvalidPolicies,
	dClient dclient.Interface,
	userInfo *v1beta1.RequestInfo,
	mutateLogPathIsDir bool,
	rclient registryclient.Client,
) (*processor.ResultCounts, []*unstructured.Unstructured, []engineapi.EngineResponse, error) {
	if vars != nil {
		vars.SetInStore()
	}
	// validate policies
	var validPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		// TODO we should return this info to the caller
		_, err := policyvalidation.Validate(pol, nil, nil, true, config.KyvernoUserName(config.KyvernoServiceAccountName()))
		if err != nil {
			log.Log.Error(err, "policy validation error")
			if strings.HasPrefix(err.Error(), "variable 'element.name'") {
				skipInvalidPolicies.invalid = append(skipInvalidPolicies.invalid, pol.GetName())
			} else {
				skipInvalidPolicies.skipped = append(skipInvalidPolicies.skipped, pol.GetName())
			}
			continue
		}
		validPolicies = append(validPolicies, pol)
	}

	var rc processor.ResultCounts
	var responses []engineapi.EngineResponse
	for _, resource := range resources {
		processor := processor.PolicyProcessor{
			Policies:             validPolicies,
			Resource:             *resource,
			MutateLogPath:        c.MutateLogPath,
			MutateLogPathIsDir:   mutateLogPathIsDir,
			Variables:            vars,
			UserInfo:             userInfo,
			PolicyReport:         c.PolicyReport,
			NamespaceSelectorMap: vars.NamespaceSelectors(),
			Stdin:                c.Stdin,
			Rc:                   &rc,
			PrintPatchResource:   true,
			Client:               dClient,
			AuditWarn:            c.AuditWarn,
			Subresources:         vars.Subresources(),
			Out:                  out,
			RegistryClient:       rclient,
		}
		ers, err := processor.ApplyPoliciesOnResource()
		if err != nil {
			return &rc, resources, responses, fmt.Errorf("failed to apply policies on resource %v (%w)", resource.GetName(), err)
		}
		responses = append(responses, ers...)
	}
	return &rc, resources, responses, nil
}

func (c *ApplyCommandConfig) loadResources(out io.Writer, policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, dClient dclient.Interface) ([]*unstructured.Unstructured, error) {
	resources, err := common.GetResourceAccordingToResourcePath(out, nil, c.ResourcePaths, c.Cluster, policies, validatingAdmissionPolicies, dClient, c.Namespace, c.PolicyReport, "")
	if err != nil {
		return resources, fmt.Errorf("failed to load resources (%w)", err)
	}
	return resources, nil
}

func (c *ApplyCommandConfig) loadPolicies(skipInvalidPolicies SkippedInvalidPolicies) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, []kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	// load policies
	var policies []kyvernov1.PolicyInterface
	var validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy

	for _, path := range c.PolicyPaths {
		isGit := source.IsGit(path)

		if isGit {
			gitSourceURL, err := url.Parse(path)
			if err != nil {
				return nil, nil, skipInvalidPolicies, nil, nil, nil, fmt.Errorf("failed to load policies (%w)", err)
			}

			pathElems := strings.Split(gitSourceURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				err := fmt.Errorf("invalid URL path %s - expected https://<any_git_source_domain>/:owner/:repository/:branch (without --git-branch flag) OR https://<any_git_source_domain>/:owner/:repository/:directory (with --git-branch flag)", gitSourceURL.Path)
				return nil, nil, skipInvalidPolicies, nil, nil, nil, fmt.Errorf("failed to parse URL (%w)", err)
			}
			gitSourceURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
			repoURL := gitSourceURL.String()
			var gitPathToYamls string
			c.GitBranch, gitPathToYamls = common.GetGitBranchOrPolicyPaths(c.GitBranch, repoURL, path)
			fs := memfs.New()
			if _, err := gitutils.Clone(repoURL, fs, c.GitBranch); err != nil {
				log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", err)
				return nil, nil, skipInvalidPolicies, nil, nil, nil, fmt.Errorf("failed to clone repository (%w)", err)
			}
			policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, nil, skipInvalidPolicies, nil, nil, nil, fmt.Errorf("failed to list YAMLs in repository (%w)", err)
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
				return nil, nil, skipInvalidPolicies, nil, nil, nil, fmt.Errorf("failed to load policies (%w)", err)
			}
			policies = append(policies, policiesFromFile...)
			validatingAdmissionPolicies = append(validatingAdmissionPolicies, admissionPoliciesFromFile...)
		}
	}

	return nil, nil, skipInvalidPolicies, nil, policies, validatingAdmissionPolicies, nil
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
			return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("failed to truncate the existing file at %s (%w)", c.MutateLogPath, err)
		}
	}
	return nil, nil, skipInvalidPolicies, nil, nil
}

func (c *ApplyCommandConfig) checkArguments() (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	var skipInvalidPolicies SkippedInvalidPolicies
	if c.ValuesFile != "" && c.Variables != nil {
		return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("pass the values either using set flag or values_file flag")
	}
	if len(c.PolicyPaths) == 0 {
		return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("require policy")
	}
	if (len(c.PolicyPaths) > 0 && c.PolicyPaths[0] == "-") && len(c.ResourcePaths) > 0 && c.ResourcePaths[0] == "-" {
		return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("a stdin pipe can be used for either policies or resources, not both")
	}
	if len(c.ResourcePaths) == 0 && !c.Cluster {
		return nil, nil, skipInvalidPolicies, nil, fmt.Errorf("resource file(s) or cluster required")
	}
	return nil, nil, skipInvalidPolicies, nil, nil
}

func printSkippedAndInvalidPolicies(out io.Writer, skipInvalidPolicies SkippedInvalidPolicies) {
	if len(skipInvalidPolicies.skipped) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, "Policies Skipped (as required variables are not provided by the user):")
		for i, policyName := range skipInvalidPolicies.skipped {
			fmt.Fprintf(out, "%d. %s\n", i+1, policyName)
		}
		fmt.Fprintln(out, divider)
	}
	if len(skipInvalidPolicies.invalid) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, "Invalid Policies:")
		for i, policyName := range skipInvalidPolicies.invalid {
			fmt.Fprintf(out, "%d. %s\n", i+1, policyName)
		}
		fmt.Fprintln(out, divider)
	}
}

func printReport(out io.Writer, engineResponses []engineapi.EngineResponse, auditWarn bool) {
	clustered, namespaced := report.ComputePolicyReports(auditWarn, engineResponses...)
	if len(clustered) > 0 || len(namespaced) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, "POLICY REPORT:")
		fmt.Fprintln(out, divider)
		report := report.MergeClusterReports(clustered)
		yamlReport, _ := yaml.Marshal(report)
		fmt.Fprintln(out, string(yamlReport))
	} else {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, "POLICY REPORT: skip generating policy report (no validate policy found/resource skipped)")
	}
}

func printViolations(out io.Writer, rc *processor.ResultCounts) {
	fmt.Fprintf(out, "\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n", rc.Pass(), rc.Fail(), rc.Warn(), rc.Error(), rc.Skip())
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
