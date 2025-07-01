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
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kyverno/kyverno-json/pkg/payload"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/autogen"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	dpolengine "github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
)

type SkippedInvalidPolicies struct {
	skipped []string
	invalid []string
}

type ApplyCommandConfig struct {
	KubeConfig            string
	Context               string
	Namespace             string
	MutateLogPath         string
	Variables             []string
	ValuesFile            string
	UserInfoPath          string
	ContextPath           string
	Cluster               bool
	PolicyReport          bool
	OutputFormat          string
	Stdin                 bool
	RegistryAccess        bool
	AuditWarn             bool
	ResourcePaths         []string
	PolicyPaths           []string
	TargetResourcePaths   []string
	GitBranch             string
	GitUsername           string
	GitPassword           string
	warnExitCode          int
	warnNoPassed          bool
	Exception             []string
	ContinueOnFail        bool
	inlineExceptions      bool
	GenerateExceptions    bool
	GeneratedExceptionTTL time.Duration
	JSONPaths             []string
	ClusterWideResources  bool
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
			color.Init(removeColor)
			applyCommandConfig.PolicyPaths = args
			rc, _, skipInvalidPolicies, responses, err := applyCommandConfig.applyCommandHelper(out)
			if err != nil {
				return err
			}
			cmd.SilenceErrors = true
			printSkippedAndInvalidPolicies(out, skipInvalidPolicies)
			if applyCommandConfig.PolicyReport {
				printReports(out, responses, applyCommandConfig.AuditWarn, applyCommandConfig.OutputFormat)
			} else if applyCommandConfig.GenerateExceptions {
				printExceptions(out, responses, applyCommandConfig.AuditWarn, applyCommandConfig.OutputFormat, applyCommandConfig.GeneratedExceptionTTL)
			} else if table {
				printTable(out, detailedResults, applyCommandConfig.AuditWarn, responses...)
			} else {
				for _, response := range responses {
					var failedRules []engineapi.RuleResponse
					resPath := fmt.Sprintf("%s/%s/%s", response.Resource.GetNamespace(), response.Resource.GetKind(), response.Resource.GetName())
					if resPath == "//" {
						resPath = "JSON payload"
					}
					for _, rule := range response.PolicyResponse.Rules {
						if rule.Status() == engineapi.RuleStatusFail {
							failedRules = append(failedRules, rule)
						}
						if rule.RuleType() == engineapi.Mutation {
							if rule.Status() == engineapi.RuleStatusSkip {
								fmt.Fprintln(out, "\nskipped mutate policy", response.Policy().GetName(), "->", "resource", resPath)
							} else if rule.Status() == engineapi.RuleStatusError {
								fmt.Fprintln(out, "\nerror while applying mutate policy", response.Policy().GetName(), "->", "resource", resPath, "\nerror: ", rule.Message())
							}
						}
					}
					if len(failedRules) > 0 {
						auditWarn := false
						if applyCommandConfig.AuditWarn && response.GetValidationFailureAction().Audit() {
							auditWarn = true
						}
						if auditWarn {
							fmt.Fprintln(out, "policy", response.Policy().GetName(), "->", "resource", resPath, "failed as audit warning:")
						} else {
							fmt.Fprintln(out, "policy", response.Policy().GetName(), "->", "resource", resPath, "failed:")
						}
						for i, rule := range failedRules {
							fmt.Fprintln(out, i+1, "-", rule.Name(), rule.Message())
						}
						fmt.Fprintln(out, "")
					}
				}
				printViolations(out, rc)
			}
			return exit(out, rc, applyCommandConfig.warnExitCode, applyCommandConfig.warnNoPassed)
		},
	}

	cmd.Flags().StringSliceVarP(&applyCommandConfig.JSONPaths, "json", "", []string{}, "Path to JSON payload files")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.ResourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.ResourcePaths, "resources", "", []string{}, "Path to resource files")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.TargetResourcePaths, "target-resource", "", []string{}, "Path to individual files containing target resources files for policies that have mutate existing")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.TargetResourcePaths, "target-resources", "", []string{}, "Path to a directory containing target resources files for policies that have mutate existing")
	cmd.Flags().BoolVarP(&applyCommandConfig.Cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&applyCommandConfig.MutateLogPath, "output", "o", "", "Prints the mutated/generated resources in provided file/directory")
	// currently `set` flag supports variable for single policy applied on single resource
	cmd.Flags().StringVarP(&applyCommandConfig.UserInfoPath, "userinfo", "u", "", "Admission Info including Roles, Cluster Roles and Subjects")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Variables, "set", "s", nil, "Variables that are required")
	cmd.Flags().StringVarP(&applyCommandConfig.ValuesFile, "values-file", "f", "", "File containing values for policy variables")
	cmd.Flags().StringVarP(&applyCommandConfig.ContextPath, "context-file", "", "", "File containing context data for CEL policies")
	cmd.Flags().BoolVarP(&applyCommandConfig.PolicyReport, "policy-report", "p", false, "Generates policy report when passed (default policyviolation)")
	cmd.Flags().StringVarP(&applyCommandConfig.OutputFormat, "output-format", "", "yaml", "Specifies the policy report format (json or yaml). Default: yaml.")
	cmd.Flags().StringVarP(&applyCommandConfig.Namespace, "namespace", "n", "", "Optional Policy parameter passed with cluster flag")
	cmd.Flags().BoolVarP(&applyCommandConfig.Stdin, "stdin", "i", false, "Optional mutate policy parameter to pipe directly through to kubectl")
	cmd.Flags().BoolVar(&applyCommandConfig.RegistryAccess, "registry", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().StringVar(&applyCommandConfig.KubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&applyCommandConfig.Context, "context", "", "The name of the kubeconfig context to use")
	cmd.Flags().StringVarP(&applyCommandConfig.GitBranch, "git-branch", "b", "", "test git repository branch")
	cmd.Flags().StringVar(&applyCommandConfig.GitUsername, "username", "", "Username for connecting to git repository")
	cmd.Flags().StringVar(&applyCommandConfig.GitPassword, "password", "", "Password for connecting to git repository")
	cmd.Flags().BoolVar(&applyCommandConfig.AuditWarn, "audit-warn", false, "If set to true, will flag audit policies as warnings instead of failures")
	cmd.Flags().IntVar(&applyCommandConfig.warnExitCode, "warn-exit-code", 0, "Set the exit code for warnings; if failures or errors are found, will exit 1")
	cmd.Flags().BoolVar(&applyCommandConfig.warnNoPassed, "warn-no-pass", false, "Specify if warning exit code should be raised if no objects satisfied a policy; can be used together with --warn-exit-code flag")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	cmd.Flags().BoolVar(&detailedResults, "detailed-results", false, "If set to true, display detailed results")
	cmd.Flags().BoolVarP(&table, "table", "t", false, "Show results in table format")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Exception, "exception", "e", nil, "Policy exception to be considered when evaluating policies against resources")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Exception, "exceptions", "", nil, "Policy exception to be considered when evaluating policies against resources")
	cmd.Flags().BoolVar(&applyCommandConfig.ContinueOnFail, "continue-on-fail", false, "If set to true, will continue to apply policies on the next resource upon failure to apply to the current resource instead of exiting out")
	cmd.Flags().BoolVarP(&applyCommandConfig.inlineExceptions, "exceptions-with-resources", "", false, "Evaluate policy exceptions from the resources path")
	cmd.Flags().BoolVarP(&applyCommandConfig.GenerateExceptions, "generate-exceptions", "", false, "Generate policy exceptions for each violation")
	cmd.Flags().DurationVarP(&applyCommandConfig.GeneratedExceptionTTL, "generated-exception-ttl", "", time.Hour*24*30, "Default TTL for generated exceptions")
	cmd.Flags().BoolVarP(&applyCommandConfig.ClusterWideResources, "cluster-wide-resources", "", false, "If set to true, will apply policies to cluster-wide resources")
	return cmd
}

func (c *ApplyCommandConfig) applyCommandHelper(out io.Writer) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	var skippedInvalidPolicies SkippedInvalidPolicies
	err := c.checkArguments()
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	mutateLogPathIsDir, err := c.getMutateLogPathIsDir()
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	if err := c.cleanPreviousContent(mutateLogPathIsDir); err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	var userInfo *kyvernov2.RequestInfo
	if c.UserInfoPath != "" {
		info, err := userinfo.Load(nil, c.UserInfoPath, "")
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("failed to load request info (%w)", err)
		}
		deprecations.CheckUserInfo(out, c.UserInfoPath, info)
		userInfo = &info.RequestInfo
	}
	variables, err := variables.New(out, nil, "", c.ValuesFile, nil, c.Variables...)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("failed to decode yaml (%w)", err)
	}
	var store store.Store

	kpols, vaps, vapBindings, maps, mapBindings, vps, ivps, gps, dps, mps, err := c.loadPolicies()
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	genericPolicies := make([]engineapi.GenericPolicy, 0, len(kpols)+len(vaps)+len(vps)+len(ivps)+len(gps)+len(dps))
	for _, pol := range kpols {
		genericPolicies = append(genericPolicies, engineapi.NewKyvernoPolicy(pol))
	}
	for _, pol := range vaps {
		genericPolicies = append(genericPolicies, engineapi.NewValidatingAdmissionPolicy(&pol))
	}
	for _, pol := range maps {
		genericPolicies = append(genericPolicies, engineapi.NewMutatingAdmissionPolicy(&pol))
	}
	for _, pol := range vps {
		genericPolicies = append(genericPolicies, engineapi.NewValidatingPolicy(&pol))
	}
	for _, pol := range ivps {
		genericPolicies = append(genericPolicies, engineapi.NewImageValidatingPolicy(&pol))
	}
	for _, pol := range gps {
		genericPolicies = append(genericPolicies, engineapi.NewGeneratingPolicy(&pol))
	}
	for _, pol := range dps {
		genericPolicies = append(genericPolicies, engineapi.NewDeletingPolicy(&pol))
	}
	for _, pol := range mps {
		genericPolicies = append(genericPolicies, engineapi.NewMutatingPolicy(&pol))
	}
	var targetResources []*unstructured.Unstructured
	if len(c.TargetResourcePaths) > 0 {
		targetResources, _, err = c.loadResources(out, c.TargetResourcePaths, genericPolicies, nil)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, err
		}
	}
	dClient, err := c.initStoreAndClusterClient(&store, targetResources...)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}

	resources, jsonPayloads, err := c.loadResources(out, c.ResourcePaths, genericPolicies, dClient)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	var exceptions []*kyvernov2.PolicyException
	var celexceptions []*policiesv1alpha1.PolicyException
	if c.inlineExceptions {
		exceptions = exception.SelectFrom(resources)
	} else {
		results, err := exception.Load(c.Exception...)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("Error: failed to load exceptions (%s)", err)
		}
		if results != nil {
			exceptions = results.Exceptions
			celexceptions = results.CELExceptions
		}
	}
	if !c.Stdin && !c.PolicyReport && !c.GenerateExceptions {
		var policyRulesCount int
		for _, policy := range kpols {
			policyRulesCount += len(autogen.Default.ComputeRules(policy, ""))
		}
		policyRulesCount += len(vaps)
		policyRulesCount += len(vps)
		policyRulesCount += len(ivps)
		policyRulesCount += len(maps)
		policyRulesCount += len(gps)
		policyRulesCount += len(dps)
		policyRulesCount += len(mps)
		exceptionsCount := len(exceptions)
		exceptionsCount += len(celexceptions)
		resourceCount := len(resources) + len(jsonPayloads)
		if exceptionsCount > 0 {
			fmt.Fprintf(out, "\nApplying %d policy rule(s) to %d resource(s) with %d exception(s)...\n", policyRulesCount, resourceCount, exceptionsCount)
		} else {
			fmt.Fprintf(out, "\nApplying %d policy rule(s) to %d resource(s)...\n", policyRulesCount, resourceCount)
		}
	}
	rc, resources1, responses1, err := c.applyPolicies(
		out,
		&store,
		variables,
		kpols,
		vaps,
		vapBindings,
		vps,
		gps,
		mps,
		maps,
		mapBindings,
		resources,
		jsonPayloads,
		exceptions,
		celexceptions,
		&skippedInvalidPolicies,
		dClient,
		userInfo,
		mutateLogPathIsDir,
	)
	if err != nil {
		return rc, resources1, skippedInvalidPolicies, responses1, err
	}
	responses4, err := c.applyImageValidatingPolicies(ivps, jsonPayloads, resources1, celexceptions, variables.Namespace, userInfo, rc, dClient)
	if err != nil {
		return rc, resources1, skippedInvalidPolicies, responses4, err
	}

	responses5, err := c.applyDeletingPolicies(dps, resources1, celexceptions, variables.Namespace, rc, dClient, "resource")
	if err != nil {
		return rc, resources1, skippedInvalidPolicies, responses4, err
	}

	responses6, err := c.applyDeletingPolicies(dps, jsonPayloads, celexceptions, variables.Namespace, rc, dClient, "json")
	if err != nil {
		return rc, resources1, skippedInvalidPolicies, responses4, err
	}

	var responses []engineapi.EngineResponse
	responses = append(responses, responses1...)
	responses = append(responses, responses4...)
	responses = append(responses, responses5...)
	responses = append(responses, responses6...)
	return rc, resources1, skippedInvalidPolicies, responses, nil
}

func (c *ApplyCommandConfig) getMutateLogPathIsDir() (bool, error) {
	mutateLogPathIsDir, err := checkMutateLogPath(c.MutateLogPath)
	if err != nil {
		return false, fmt.Errorf("failed to create file/folder (%w)", err)
	}
	return mutateLogPathIsDir, nil
}

func (c *ApplyCommandConfig) applyPolicies(
	out io.Writer,
	store *store.Store,
	vars *variables.Variables,
	policies []kyvernov1.PolicyInterface,
	vaps []admissionregistrationv1.ValidatingAdmissionPolicy,
	vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	vpols []policiesv1alpha1.ValidatingPolicy,
	gpols []policiesv1alpha1.GeneratingPolicy,
	mpols []policiesv1alpha1.MutatingPolicy,
	maps []admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	mapBindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	resources []*unstructured.Unstructured,
	jsonPayloads []*unstructured.Unstructured,
	exceptions []*kyvernov2.PolicyException,
	celExceptions []*policiesv1alpha1.PolicyException,
	skipInvalidPolicies *SkippedInvalidPolicies,
	dClient dclient.Interface,
	userInfo *kyvernov2.RequestInfo,
	mutateLogPathIsDir bool,
) (*processor.ResultCounts, []*unstructured.Unstructured, []engineapi.EngineResponse, error) {
	if vars != nil {
		vars.SetInStore(store)
	}
	var rc processor.ResultCounts
	// validate policies
	validPolicies := make([]kyvernov1.PolicyInterface, 0, len(policies))
	for _, pol := range policies {
		// TODO we should return this info to the caller
		sa := config.KyvernoUserName(config.KyvernoServiceAccountName())
		_, err := policyvalidation.Validate(pol, nil, nil, true, sa, sa)
		if err != nil {
			log.Log.Error(err, "policy validation error")
			rc.IncrementError(1)
			if strings.HasPrefix(err.Error(), "variable 'element.name'") {
				skipInvalidPolicies.invalid = append(skipInvalidPolicies.invalid, pol.GetName())
			} else {
				skipInvalidPolicies.skipped = append(skipInvalidPolicies.skipped, pol.GetName())
			}
			continue
		}
		validPolicies = append(validPolicies, pol)
	}
	var responses []engineapi.EngineResponse
	for _, resource := range resources {
		processor := processor.PolicyProcessor{
			Store:                             store,
			Policies:                          validPolicies,
			ValidatingAdmissionPolicies:       vaps,
			ValidatingAdmissionPolicyBindings: vapBindings,
			ValidatingPolicies:                vpols,
			GeneratingPolicies:                gpols,
			MutatingPolicies:                  mpols,
			MutatingAdmissionPolicies:         maps,
			MutatingAdmissionPolicyBindings:   mapBindings,
			Resource:                          *resource,
			PolicyExceptions:                  exceptions,
			CELExceptions:                     celExceptions,
			MutateLogPath:                     c.MutateLogPath,
			MutateLogPathIsDir:                mutateLogPathIsDir,
			Variables:                         vars,
			ContextPath:                       c.ContextPath,
			UserInfo:                          userInfo,
			PolicyReport:                      c.PolicyReport,
			NamespaceSelectorMap:              vars.NamespaceSelectors(),
			Stdin:                             c.Stdin,
			Rc:                                &rc,
			PrintPatchResource:                true,
			Cluster:                           c.Cluster,
			Client:                            dClient,
			AuditWarn:                         c.AuditWarn,
			Subresources:                      vars.Subresources(),
			Out:                               out,
		}
		ers, err := processor.ApplyPoliciesOnResource()
		if err != nil {
			if c.ContinueOnFail {
				log.Log.V(2).Info(fmt.Sprintf("failed to apply policies on resource %s (%s)\n", resource.GetName(), err.Error()))
				continue
			}
			return &rc, resources, responses, fmt.Errorf("failed to apply policies on resource %s (%w)", resource.GetName(), err)
		}
		responses = append(responses, ers...)
	}
	for _, resource := range jsonPayloads {
		processor := processor.PolicyProcessor{
			Store:                             store,
			Policies:                          validPolicies,
			ValidatingAdmissionPolicies:       vaps,
			ValidatingAdmissionPolicyBindings: vapBindings,
			ValidatingPolicies:                vpols,
			MutatingPolicies:                  mpols,
			MutatingAdmissionPolicies:         maps,
			MutatingAdmissionPolicyBindings:   mapBindings,
			JsonPayload:                       *resource,
			PolicyExceptions:                  exceptions,
			CELExceptions:                     celExceptions,
			MutateLogPath:                     c.MutateLogPath,
			MutateLogPathIsDir:                mutateLogPathIsDir,
			Variables:                         vars,
			ContextPath:                       c.ContextPath,
			UserInfo:                          userInfo,
			PolicyReport:                      c.PolicyReport,
			NamespaceSelectorMap:              vars.NamespaceSelectors(),
			Stdin:                             c.Stdin,
			Rc:                                &rc,
			PrintPatchResource:                true,
			Cluster:                           c.Cluster,
			Client:                            dClient,
			AuditWarn:                         c.AuditWarn,
			Subresources:                      vars.Subresources(),
			Out:                               out,
		}
		ers, err := processor.ApplyPoliciesOnResource()
		if err != nil {
			if c.ContinueOnFail {
				log.Log.V(2).Info(fmt.Sprintf("failed to apply policies on resource %s (%s)\n", resource.GetName(), err.Error()))
				continue
			}
			return &rc, resources, responses, fmt.Errorf("failed to apply policies on resource %s (%w)", resource.GetName(), err)
		}
		responses = append(responses, ers...)
	}
	for _, policy := range validPolicies {
		if policy.GetNamespace() == "" && policy.GetKind() == "Policy" {
			log.Log.V(3).Info(fmt.Sprintf("Policy %s has no namespace detected. Ensure that namespaced policies are correctly loaded.", policy.GetNamespace()))
		}
	}
	return &rc, resources, responses, nil
}

func (c *ApplyCommandConfig) applyImageValidatingPolicies(
	ivps []policiesv1alpha1.ImageValidatingPolicy,
	jsonPayloads []*unstructured.Unstructured,
	resources []*unstructured.Unstructured,
	celExceptions []*policiesv1alpha1.PolicyException,
	namespaceProvider func(string) *corev1.Namespace,
	userInfo *kyvernov2.RequestInfo,
	rc *processor.ResultCounts,
	dclient dclient.Interface,
) ([]engineapi.EngineResponse, error) {
	if len(ivps) == 0 {
		return nil, nil
	}
	provider, err := ivpolengine.NewProvider(ivps, celExceptions)
	if err != nil {
		return nil, err
	}
	var lister k8scorev1.SecretInterface
	if dclient != nil {
		lister = dclient.GetKubeClient().CoreV1().Secrets("")
	}
	engine := ivpolengine.NewEngine(
		provider,
		namespaceProvider,
		matching.NewMatcher(),
		lister,
		[]imagedataloader.Option{imagedataloader.WithLocalCredentials(c.RegistryAccess)},
	)

	restMapper, err := utils.GetRESTMapper(dclient, !c.Cluster)
	if err != nil {
		return nil, err
	}

	contextProvider, err := processor.NewContextProvider(dclient, restMapper, c.ContextPath, c.RegistryAccess, !c.Cluster)
	if err != nil {
		return nil, err
	}

	responses := make([]engineapi.EngineResponse, 0)
	for _, resource := range resources {
		gvk := resource.GroupVersionKind()
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			log.Log.Error(err, "failed to map gvk to gvr", "gkv", gvk)
			if c.ContinueOnFail {
				continue
			}
			return responses, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
		}
		gvr := mapping.Resource
		var user authenticationv1.UserInfo
		if userInfo != nil {
			user = userInfo.AdmissionUserInfo
		}
		request := celengine.Request(
			contextProvider,
			resource.GroupVersionKind(),
			gvr,
			"",
			resource.GetName(),
			resource.GetNamespace(),
			admissionv1.Create,
			user,
			resource,
			nil,
			false,
			nil,
		)
		engineResponse, _, err := engine.HandleMutating(context.TODO(), request, nil)
		if err != nil {
			if c.ContinueOnFail {
				fmt.Printf("failed to apply image validating policies on resource %s (%v)\n", resource.GetName(), err)
				continue
			}
			return responses, fmt.Errorf("failed to apply image validating policies on resource %s (%w)", resource.GetName(), err)
		}
		resp := engineapi.EngineResponse{
			Resource:       *resource,
			PolicyResponse: engineapi.PolicyResponse{},
		}

		for _, r := range engineResponse.Policies {
			resp.PolicyResponse.Rules = []engineapi.RuleResponse{r.Result}
			resp = resp.WithPolicy(engineapi.NewImageValidatingPolicy(r.Policy))
			rc.AddValidatingPolicyResponse(resp)
			responses = append(responses, resp)
		}
	}

	ivpols := make([]*eval.CompiledImageValidatingPolicy, 0)
	pMap := make(map[string]*policiesv1alpha1.ImageValidatingPolicy)
	for i := range ivps {
		p := ivps[i]
		pMap[p.GetName()] = &p
		ivpols = append(ivpols, &eval.CompiledImageValidatingPolicy{Policy: &p})
	}
	for _, json := range jsonPayloads {
		result, err := eval.Evaluate(context.TODO(), ivpols, json.Object, nil, nil, nil)
		if err != nil {
			if c.ContinueOnFail {
				fmt.Printf("failed to apply image validating policies on JSON payload: %v\n", err)
				continue
			}
			return responses, fmt.Errorf("failed to apply image validating policies on JSON payload: %w", err)
		}
		resp := engineapi.EngineResponse{
			Resource:       *json,
			PolicyResponse: engineapi.PolicyResponse{},
		}
		for p, rslt := range result {
			if rslt.Error != nil {
				resp.PolicyResponse.Rules = []engineapi.RuleResponse{
					*engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to evaluate policy for JSON", rslt.Error, nil),
				}
			} else if rslt.Result {
				resp.PolicyResponse.Rules = []engineapi.RuleResponse{
					*engineapi.RulePass(p, engineapi.ImageVerify, "success", nil),
				}
			} else {
				resp.PolicyResponse.Rules = []engineapi.RuleResponse{
					*engineapi.RuleFail(p, engineapi.ImageVerify, rslt.Message, rslt.AuditAnnotations),
				}
			}
			resp = resp.WithPolicy(engineapi.NewImageValidatingPolicy(pMap[p]))
			rc.AddValidatingPolicyResponse(resp)
			responses = append(responses, resp)
		}
	}
	return responses, nil
}

func (c *ApplyCommandConfig) applyDeletingPolicies(
	dps []policiesv1alpha1.DeletingPolicy,
	resources []*unstructured.Unstructured,
	celExceptions []*policiesv1alpha1.PolicyException,
	namespaceProvider func(string) *corev1.Namespace,
	rc *processor.ResultCounts,
	dclient dclient.Interface,
	payloadType string,
) ([]engineapi.EngineResponse, error) {
	provider, err := dpolengine.NewProvider(dpolcompiler.NewCompiler(), dps, celExceptions)
	if err != nil {
		return nil, err
	}

	restMapper, err := utils.GetRESTMapper(dclient, true)
	if err != nil {
		return nil, err
	}

	contextProvider, err := processor.NewContextProvider(dclient, restMapper, c.ContextPath, c.RegistryAccess, !c.Cluster)
	if err != nil {
		return nil, err
	}

	engine := dpolengine.NewEngine(namespaceProvider, restMapper, contextProvider, matching.NewMatcher())

	policies, err := provider.Fetch(context.Background())
	if err != nil {
		return nil, err
	}

	responses := make([]engineapi.EngineResponse, 0)
	for _, resource := range resources {
		for _, dpol := range policies {
			resp, err := engine.Handle(context.TODO(), dpol, *resource)
			if err != nil {
				response := engineapi.NewEngineResponse(*resource, engineapi.NewDeletingPolicy(&dpol.Policy), nil)
				response = response.WithPolicyResponse(engineapi.PolicyResponse{Rules: []engineapi.RuleResponse{
					*engineapi.NewRuleResponse(dpol.Policy.Name, engineapi.Deletion, err.Error(), engineapi.RuleStatusError, nil),
				}})

				responses = append(responses, response)
				rc.AddValidatingPolicyResponse(response)

				if c.ContinueOnFail {
					fmt.Printf("failed to apply deleting policies on %s: %v\n", payloadType, err)
					continue
				}
				return responses, fmt.Errorf("failed to apply deleting policies on %s: %w", payloadType, err)
			}

			status := engineapi.RuleStatusPass
			message := fmt.Sprintf("%s matched", payloadType)
			if !resp.Match {
				status = engineapi.RuleStatusFail
				message = fmt.Sprintf("%s did not match", payloadType)
			}

			response := engineapi.NewEngineResponse(*resource, engineapi.NewDeletingPolicy(&dpol.Policy), nil)
			response = response.WithPolicyResponse(engineapi.PolicyResponse{Rules: []engineapi.RuleResponse{
				*engineapi.NewRuleResponse(dpol.Policy.Name, engineapi.Deletion, message, status, nil),
			}})

			responses = append(responses, response)

			rc.AddValidatingPolicyResponse(response)
		}
	}

	return responses, nil
}

func (c *ApplyCommandConfig) loadResources(out io.Writer, paths []string, policies []engineapi.GenericPolicy, dClient dclient.Interface) ([]*unstructured.Unstructured, []*unstructured.Unstructured, error) {
	resources, err := common.GetResourceAccordingToResourcePath(out, nil, paths, c.Cluster, policies, dClient, c.Namespace, c.PolicyReport, c.ClusterWideResources, "")
	if err != nil {
		return resources, nil, fmt.Errorf("failed to load resources (%w)", err)
	}
	resources = test.ProcessResources(resources)
	var jsonPayloads []*unstructured.Unstructured
	if len(c.JSONPaths) > 0 {
		for _, path := range c.JSONPaths {
			payload, err := payload.Load(path)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load JSON payload (%w)", err)
			}

			jsonPayloads = append(jsonPayloads, &unstructured.Unstructured{Object: payload.(map[string]interface{})})
		}
	}
	return resources, jsonPayloads, nil
}

func (c *ApplyCommandConfig) loadPolicies() (
	[]kyvernov1.PolicyInterface,
	[]admissionregistrationv1.ValidatingAdmissionPolicy,
	[]admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	[]admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	[]admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	[]policiesv1alpha1.ValidatingPolicy,
	[]policiesv1alpha1.ImageValidatingPolicy,
	[]policiesv1alpha1.GeneratingPolicy,
	[]policiesv1alpha1.DeletingPolicy,
	[]policiesv1alpha1.MutatingPolicy,
	error,
) {
	// load policies
	var policies []kyvernov1.PolicyInterface
	var vaps []admissionregistrationv1.ValidatingAdmissionPolicy
	var vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	var vps []policiesv1alpha1.ValidatingPolicy
	var maps []admissionregistrationv1alpha1.MutatingAdmissionPolicy
	var mapBindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
	var ivps []policiesv1alpha1.ImageValidatingPolicy
	var gps []policiesv1alpha1.GeneratingPolicy
	var dps []policiesv1alpha1.DeletingPolicy
	var mps []policiesv1alpha1.MutatingPolicy
	for _, path := range c.PolicyPaths {
		isGit := source.IsGit(path)
		if isGit {
			gitSourceURL, err := url.Parse(path)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to load policies (%w)", err)
			}
			pathElems := strings.Split(gitSourceURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				err := fmt.Errorf("invalid URL path %s - expected https://<any_git_source_domain>/:owner/:repository/:branch (without --git-branch flag) OR https://<any_git_source_domain>/:owner/:repository/:directory (with --git-branch flag)", gitSourceURL.Path)
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to parse URL (%w)", err)
			}
			gitSourceURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
			repoURL := gitSourceURL.String()
			var gitPathToYamls string
			c.GitBranch, gitPathToYamls = common.GetGitBranchOrPolicyPaths(c.GitBranch, repoURL, path)
			fs := memfs.New()
			auth := &http.BasicAuth{
				Username: c.GitUsername,
				Password: c.GitPassword,
			}
			if _, err := gitutils.Clone(repoURL, fs, c.GitBranch, *auth); err != nil {
				log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", err)
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to clone repository (%w)", err)
			}
			policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to list YAMLs in repository (%w)", err)
			}
			for _, policyYaml := range policyYamls {
				loaderResults, err := policy.Load(fs, "", policyYaml)
				if loaderResults != nil && loaderResults.NonFatalErrors != nil {
					for _, err := range loaderResults.NonFatalErrors {
						log.Log.Error(err.Error, "Non-fatal parsing error for single document")
					}
				}
				if err != nil {
					continue
				}
				policies = append(policies, loaderResults.Policies...)
				vaps = append(vaps, loaderResults.VAPs...)
				vapBindings = append(vapBindings, loaderResults.VAPBindings...)
				vps = append(vps, loaderResults.ValidatingPolicies...)
				maps = append(maps, loaderResults.MAPs...)
				mapBindings = append(mapBindings, loaderResults.MAPBindings...)
				ivps = append(ivps, loaderResults.ImageValidatingPolicies...)
				dps = append(dps, loaderResults.DeletingPolicies...)
				mps = append(mps, loaderResults.MutatingPolicies...)
			}
		} else {
			loaderResults, err := policy.Load(nil, "", path)
			if loaderResults != nil && loaderResults.NonFatalErrors != nil {
				for _, err := range loaderResults.NonFatalErrors {
					log.Log.Error(err.Error, "Non-fatal parsing error for single document")
				}
			}
			if err != nil {
				log.Log.V(3).Info("skipping invalid YAML file", "path", path, "error", err)
			} else {
				policies = append(policies, loaderResults.Policies...)
				vaps = append(vaps, loaderResults.VAPs...)
				vapBindings = append(vapBindings, loaderResults.VAPBindings...)
				vps = append(vps, loaderResults.ValidatingPolicies...)
				maps = append(maps, loaderResults.MAPs...)
				mapBindings = append(mapBindings, loaderResults.MAPBindings...)
				ivps = append(ivps, loaderResults.ImageValidatingPolicies...)
				gps = append(gps, loaderResults.GeneratingPolicies...)
				dps = append(dps, loaderResults.DeletingPolicies...)
				mps = append(mps, loaderResults.MutatingPolicies...)
			}
		}
		for _, policy := range policies {
			if policy.GetNamespace() == "" && policy.GetKind() == "Policy" {
				log.Log.V(3).Info(fmt.Sprintf("Namespace is empty for a namespaced Policy %s. This might cause incorrect report generation.", policy.GetName()))
			}
		}
	}
	return policies, vaps, vapBindings, maps, mapBindings, vps, ivps, gps, dps, mps, nil
}

func (c *ApplyCommandConfig) initStoreAndClusterClient(store *store.Store, targetResources ...*unstructured.Unstructured) (dclient.Interface, error) {
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
			return nil, err
		}
		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}
		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}
		metadataClient, err := metadata.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}
		dClient, err = dclient.NewClient(context.Background(), dynamicClient, kubeClient, 15*time.Minute, false, metadataClient)
		if err != nil {
			return nil, err
		}
	}
	if len(targetResources) > 0 && !c.Cluster {
		var targets []runtime.Object
		for _, t := range targetResources {
			targets = append(targets, t)
		}
		dClient, err = dclient.NewFakeClient(runtime.NewScheme(), map[schema.GroupVersionResource]string{}, targets...)
		dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
		if err != nil {
			return nil, err
		}
	}
	return dClient, err
}

func (c *ApplyCommandConfig) cleanPreviousContent(mutateLogPathIsDir bool) error {
	// empty the previous contents of the file just in case if the file already existed before with some content(so as to perform overwrites)
	// the truncation of files for the case when mutateLogPath is dir, is handled under pkg/kyverno/apply/common.go
	if !mutateLogPathIsDir && c.MutateLogPath != "" {
		c.MutateLogPath = filepath.Clean(c.MutateLogPath)
		// Necessary for us to include the file via variable as it is part of the CLI.
		_, err := os.OpenFile(c.MutateLogPath, os.O_TRUNC|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return fmt.Errorf("failed to truncate the existing file at %s (%w)", c.MutateLogPath, err)
		}
	}
	return nil
}

func (c *ApplyCommandConfig) checkArguments() error {
	if c.ValuesFile != "" && c.Variables != nil {
		return fmt.Errorf("pass the values either using set flag or values_file flag")
	}
	if len(c.PolicyPaths) == 0 {
		return fmt.Errorf("require policy")
	}
	if (len(c.PolicyPaths) > 0 && c.PolicyPaths[0] == "-") && len(c.ResourcePaths) > 0 && c.ResourcePaths[0] == "-" {
		return fmt.Errorf("a stdin pipe can be used for either policies or resources, not both")
	}
	if len(c.ResourcePaths) != 0 && len(c.JSONPaths) != 0 {
		return fmt.Errorf("both resource and json files can not be used together, use one or the other")
	}
	if len(c.ResourcePaths) == 0 && len(c.JSONPaths) == 0 && !c.Cluster {
		return fmt.Errorf("resource file(s) or cluster required")
	}
	return nil
}

type WarnExitCodeError struct {
	ExitCode int
}

func (w WarnExitCodeError) Error() string {
	return fmt.Sprintf("exit as warnExitCode is %d", w.ExitCode)
}

func exit(out io.Writer, rc *processor.ResultCounts, warnExitCode int, warnNoPassed bool) error {
	if rc.Fail > 0 {
		return fmt.Errorf("exit as there are policy violations")
	} else if rc.Error > 0 {
		return fmt.Errorf("exit as there are policy errors")
	} else if rc.Warn > 0 && warnExitCode != 0 {
		fmt.Printf("exit as warnExitCode is %d", warnExitCode)
		return WarnExitCodeError{
			ExitCode: warnExitCode,
		}
	} else if rc.Pass == 0 && warnNoPassed {
		fmt.Println(out, "exit as no objects satisfied policy")
		return WarnExitCodeError{
			ExitCode: warnExitCode,
		}
	}
	return nil
}
