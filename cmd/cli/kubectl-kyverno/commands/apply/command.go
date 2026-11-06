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

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	authzhttp "github.com/kyverno/kyverno-authz/pkg/cel/libs/authz/http"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/payload"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/cli/loader"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

type SkippedInvalidPolicies struct {
	skipped []PolicyDiagnostic
	invalid []PolicyDiagnostic
}

type PolicyDiagnostic struct {
	name   string
	reason string
}

type ApplyCommandConfig struct {
	KubeConfig                string
	Context                   string
	Namespace                 string
	MutateLogPath             string
	OutputDir                 string
	OutputFile                string
	Variables                 []string
	ValuesFile                string
	UserInfoPath              string
	ContextPath               string
	Cluster                   bool
	PolicyReport              bool
	OutputFormat              string
	Stdin                     bool
	RegistryAccess            bool
	AuditWarn                 bool
	Explain                   bool
	ResourcePaths             []string
	PolicyPaths               []string
	TargetResourcePaths       []string
	ParamResources            []string
	GitBranch                 string
	GitUsername               string
	GitPassword               string
	warnExitCode              int
	warnNoPassed              bool
	warningsAsErrors          bool
	Exception                 []string
	ContinueOnFail            bool
	inlineExceptions          bool
	exceptionsWithinResources bool
	exceptionsWithinPolicies  bool
	GenerateExceptions        bool
	GeneratedExceptionTTL     time.Duration
	JSONPaths                 []string
	HTTPPayloadPaths          []string
	EnvoyPayloadPaths         []string
	ClusterWideResources      bool
	Concurrent                int
	BatchSize                 int
	ContinueOnError           bool
	ShowPerformance           bool
	CrdPaths                  []string
	deprecationWarnings       []string
	// Cloner is an optional function for cloning git repositories.
	// If nil, defaults to gitutils.Clone. Tests can inject a fake
	// to avoid real network calls while still exercising the git-URL
	// policy loading code path.
	Cloner gitutils.CloneFunc
}

// cloneRepo clones a git repository using the configured Cloner function.
// If no Cloner is set, it falls back to the default gitutils.Clone which
// includes a timeout to prevent indefinite hangs.
func (c *ApplyCommandConfig) cloneRepo(repoURL string, fs billy.Filesystem, branch string, auth http.BasicAuth) (*git.Repository, error) {
	cloneFn := gitutils.CloneFunc(gitutils.Clone)
	if c.Cloner != nil {
		cloneFn = c.Cloner
	}
	return cloneFn(repoURL, fs, branch, auth)
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
			if applyCommandConfig.PolicyReport || applyCommandConfig.GenerateExceptions || applyCommandConfig.Stdin {
				removeColor = true
			}
			color.Init(removeColor)
			applyCommandConfig.PolicyPaths = args
			table, err = applyCommandConfig.resolveOutputFlags(cmd, table, detailedResults)
			if err != nil {
				return err
			}
			rc, _, skipInvalidPolicies, responses, err := applyCommandConfig.applyCommandHelper(cmd.Context(), out)
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
								fmt.Fprintln(out, "\n"+color.BoldYellow("skipped mutate policy"), color.Policy("", response.Policy().GetName()), "->", "resource", resPath)
							} else if rule.Status() == engineapi.RuleStatusError {
								fmt.Fprintln(out, "\n"+color.BoldRed("error while applying mutate policy"), color.Policy("", response.Policy().GetName()), "->", "resource", resPath, "\n"+color.BoldRed("error:")+" ", rule.Message())
							}
						}
						if rule.RuleType() == engineapi.Generation {
							if rule.Status() == engineapi.RuleStatusError {
								fmt.Fprintln(out, "\n"+color.BoldRed("error while applying generate policy"), color.Policy("", response.Policy().GetName()), "->", "resource", resPath, "\n"+color.BoldRed("error:"), rule.Message())
							}
						}
					}
					if len(failedRules) > 0 {
						fmt.Fprintln(out, "policy", color.Policy("", response.Policy().GetName()), "->", "resource", resPath, color.BoldRed("failed:"))
						for i, rule := range failedRules {
							msg := rule.Message()
							if msg == "" {
								msg = "validation failed"
							}
							fmt.Fprintln(out, i+1, "-", color.Rule(rule.Name()), msg)
						}
					}
				}
				printViolations(out, rc)
			}
			return exit(out, rc, applyCommandConfig.warnExitCode, applyCommandConfig.warnNoPassed)
		},
	}

	cmd.Flags().StringSliceVarP(&applyCommandConfig.JSONPaths, "json", "", []string{}, "Path to JSON payload files")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.HTTPPayloadPaths, "http-payload", "", []string{}, "Path to HTTP check request payload files (JSON)")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.EnvoyPayloadPaths, "envoy-payload", "", []string{}, "Path to Envoy check request payload files (JSON)")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.ResourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.ResourcePaths, "resources", "", []string{}, "Path to resource files")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.TargetResourcePaths, "target-resource", "", []string{}, "Path to individual files containing target resources files for policies that have mutate existing")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.TargetResourcePaths, "target-resources", "", []string{}, "Path to a directory containing target resources files for policies that have mutate existing")
	cmd.Flags().BoolVarP(&applyCommandConfig.Cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&applyCommandConfig.MutateLogPath, "output", "o", "", "Prints the mutated/generated resources in provided file/directory")
	cmd.Flags().StringVar(&applyCommandConfig.OutputDir, "output-dir", "", "Directory for mutated/generated resources")
	cmd.Flags().StringVar(&applyCommandConfig.OutputFile, "output-file", "", "File for mutated/generated resources")
	_ = cmd.Flags().MarkDeprecated("output", "use --output-dir or --output-file instead; -o/--output will select the output format in a future release")
	cmd.MarkFlagsMutuallyExclusive("output", "output-dir", "output-file")
	// currently `set` flag supports variable for single policy applied on single resource
	cmd.Flags().StringVarP(&applyCommandConfig.UserInfoPath, "userinfo", "u", "", "Admission Info including Roles, Cluster Roles and Subjects")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Variables, "set", "s", nil, "Variables that are required")
	cmd.Flags().StringVarP(&applyCommandConfig.ValuesFile, "values-file", "f", "", "File containing values for policy variables")
	cmd.Flags().StringVarP(&applyCommandConfig.ContextPath, "context-file", "", "", "File containing context data for CEL policies")
	cmd.Flags().BoolVarP(&applyCommandConfig.PolicyReport, "policy-report", "p", false, "Generates policy report when passed (default policyviolation)")
	cmd.Flags().StringVarP(&applyCommandConfig.OutputFormat, "output-format", "", "yaml", "Output format: json or yaml for policy reports/exceptions, or table for evaluation results")
	cmd.Flags().StringVarP(&applyCommandConfig.Namespace, "namespace", "n", "", "Optional Policy parameter passed with cluster flag")
	cmd.Flags().BoolVarP(&applyCommandConfig.Stdin, "stdin", "i", false, "Optional mutate policy parameter to pipe directly through to kubectl")
	cmd.Flags().BoolVar(&applyCommandConfig.RegistryAccess, "registry", false, "If set to true, access the image registry using local docker credentials to populate external data")
	cmd.Flags().StringVar(&applyCommandConfig.KubeConfig, "kubeconfig", "", "path to kubeconfig file with authorization and master location information")
	cmd.Flags().StringVar(&applyCommandConfig.Context, "context", "", "The name of the kubeconfig context to use")
	cmd.Flags().StringVarP(&applyCommandConfig.GitBranch, "git-branch", "b", "", "test git repository branch")
	cmd.Flags().StringVar(&applyCommandConfig.GitUsername, "username", "", "Username for connecting to git repository")
	cmd.Flags().StringVar(&applyCommandConfig.GitPassword, "password", "", "Password for connecting to git repository")
	cmd.Flags().BoolVar(&applyCommandConfig.Explain, "explain", false, "Print how each ValidatingPolicy, MutatingPolicy, GeneratingPolicy, ImageValidatingPolicy and DeletingPolicy reached its result: whether it applied, its match conditions, variables, the images it checked, and its validations, the mutations it ran, the generate entries it ran or its delete conditions. The trace prints the values the expressions read, including resource fields and variables, so a policy that reads a Secret's data prints that data; treat the output as sensitive")
	cmd.Flags().BoolVar(&applyCommandConfig.AuditWarn, "audit-warn", false, "Currently has no effect; ValidatingPolicy audit support is tracked by https://github.com/kyverno/kyverno/issues/16485")
	cmd.Flags().IntVar(&applyCommandConfig.warnExitCode, "warn-exit-code", 0, "Set the exit code for warnings; if failures or errors are found, will exit 1")
	cmd.Flags().BoolVar(&applyCommandConfig.warnNoPassed, "warn-no-pass", false, "Specify if warning exit code should be raised if no objects satisfied a policy; can be used together with --warn-exit-code flag")
	cmd.Flags().BoolVar(&applyCommandConfig.warningsAsErrors, "warnings-as-errors", false, "Treat deprecation warnings as errors")
	cmd.Flags().BoolVar(&removeColor, "remove-color", false, "Remove any color from output")
	cmd.Flags().BoolVar(&detailedResults, "detailed-results", false, "If set to true, display detailed results")
	cmd.Flags().BoolVarP(&table, "table", "t", false, "Show results in table format")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.ParamResources, "parameter-resource", "", []string{}, "Path to resource files that act as ValidatingAdmissionPolicy/MutatingAdmissionPolicy parameters")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Exception, "exception", "e", nil, "Policy exception to be considered when evaluating policies against resources")
	cmd.Flags().StringSliceVarP(&applyCommandConfig.Exception, "exceptions", "", nil, "Policy exception to be considered when evaluating policies against resources")
	cmd.Flags().BoolVar(&applyCommandConfig.ContinueOnFail, "continue-on-fail", false, "If set to true, will continue to apply policies on the next resource upon failure to apply to the current resource instead of exiting out")
	cmd.Flags().BoolVarP(&applyCommandConfig.inlineExceptions, "exceptions-with-resources", "", false, "Evaluate policy exceptions from the resources path")
	cmd.Flags().BoolVarP(&applyCommandConfig.exceptionsWithinResources, "exceptions-within-resources", "", false, "Evaluate policy exceptions from the resources path")
	cmd.Flags().BoolVarP(&applyCommandConfig.exceptionsWithinPolicies, "exceptions-within-policies", "", false, "Evaluate policy exceptions from the policies path")
	cmd.Flags().BoolVarP(&applyCommandConfig.GenerateExceptions, "generate-exceptions", "", false, "Generate policy exceptions for each violation")
	cmd.Flags().DurationVarP(&applyCommandConfig.GeneratedExceptionTTL, "generated-exception-ttl", "", time.Hour*24*30, "Default TTL for generated exceptions")
	cmd.Flags().BoolVarP(&applyCommandConfig.ClusterWideResources, "cluster-wide-resources", "", false, "If set to true, will apply policies to cluster-wide resources")

	cmd.Flags().IntVar(&applyCommandConfig.Concurrent, "concurrent", 1, "Number of concurrent workers for resource loading")
	cmd.Flags().IntVar(&applyCommandConfig.BatchSize, "batch-size", 100, "Number of resources to fetch per API call")
	cmd.Flags().BoolVar(&applyCommandConfig.ContinueOnError, "continue-on-error", true, "Continue processing despite resource loading errors")
	cmd.Flags().BoolVar(&applyCommandConfig.ShowPerformance, "show-performance", false, "Show resource loading performance metrics")
	cmd.Flags().StringSliceVar(&applyCommandConfig.CrdPaths, "crd-paths", []string{}, "List of paths to CRD files to be used for apply command")
	cmd.Flags().StringSliceVar(&applyCommandConfig.CrdPaths, "crd-path", nil, "Deprecated: use --crd-paths instead")
	_ = cmd.Flags().MarkDeprecated("crd-path", "use --crd-paths instead")
	return cmd
}

func (c *ApplyCommandConfig) applyCommandHelper(ctx context.Context, out io.Writer) (*processor.ResultCounts, []*unstructured.Unstructured, SkippedInvalidPolicies, []engineapi.EngineResponse, error) {
	var skippedInvalidPolicies SkippedInvalidPolicies
	c.deprecationWarnings = nil
	err := c.checkArguments()
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	crdProcessor := data.NewCRDProcessor(nil)
	data.InjectProcessor(crdProcessor)

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
	if _, err := processor.NormalizeValuesOperation(variables.GlobalOperation()); err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	var store store.Store

	policySet, err := c.loadPolicies(ctx, out)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	if err := c.failOnDeprecationWarnings(); err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	genericPolicies := policySet.GenericPolicies()
	var targetResources []*unstructured.Unstructured
	if len(c.TargetResourcePaths) > 0 {
		targetResources, _, err = c.loadResources(out, c.TargetResourcePaths, genericPolicies, nil)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, err
		}
	}
	var parameterResources []*unstructured.Unstructured
	if len(c.ParamResources) > 0 {
		parameterResources, _, err = c.loadResources(out, c.ParamResources, genericPolicies, nil)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, err
		}
	}

	dClient, err := c.initStoreAndClusterClient(&store, append(targetResources, parameterResources...)...)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}

	resources, jsonPayloads, err := c.loadResources(out, c.ResourcePaths, genericPolicies, dClient)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}

	// Separate GlobalContextEntry resources from regular resources
	var gctxEntries []*kyvernov2.GlobalContextEntry
	resources, gctxEntries, err = extractGlobalContextEntries(resources)
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	c.recordGlobalContextEntryDeprecationWarnings(out, gctxEntries)

	if !c.Cluster && (len(resources)+len(targetResources)+len(parameterResources)) > 0 {
		dClient, err = createFakeClientFromResources(resources, targetResources, parameterResources)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("failed to create local resource client: %w", err)
		}
		store.AllowApiCall(true)

		if len(gctxEntries) > 0 && dClient != nil {
			gctxStore, err := buildGlobalContextStore(context.Background(), gctxEntries, dClient, jmespath.New(config.NewDefaultConfiguration(false)))
			if err != nil {
				return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("failed to build global context store: %w", err)
			}
			store.SetGlobalContextStore(gctxStore)
		}
	}

	var celExceptions []*policiesv1beta1.PolicyException
	// `kyverno apply` always hard-blocks legacy kyverno.io policy kinds -- no escape hatch, see #17485.
	if c.exceptionsWithinResources || c.inlineExceptions {
		results, err := exception.SelectFrom(resources)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("Error: failed to load exceptions (%s)", err)
		}
		celExceptions = results.CELExceptions
	} else {
		results, err := exception.Load(c.Exception...)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("Error: failed to load exceptions (%s)", err)
		}
		if results != nil {
			for _, warning := range results.Warnings {
				msg := fmt.Sprintf("Warning: %s", warning)
				fmt.Fprintln(out, msg)
				c.deprecationWarnings = append(c.deprecationWarnings, msg)
			}
			celExceptions = results.CELExceptions
		}
	}
	if err := c.failOnDeprecationWarnings(); err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}

	if c.exceptionsWithinPolicies {
		celExceptions = append(celExceptions, policySet.PolicyCelExceptions...)
	}

	if !c.Stdin && !c.PolicyReport && !c.GenerateExceptions {
		policyRulesCount := policySet.PolicyCount()
		exceptionsCount := len(celExceptions)
		resourceCount := len(resources) + len(jsonPayloads)
		if exceptionsCount > 0 {
			fmt.Fprintf(out, "\nApplying %d policy rule(s) to %d resource(s) with %d exception(s)...\n", policyRulesCount, resourceCount, exceptionsCount)
		} else {
			fmt.Fprintf(out, "\nApplying %d policy rule(s) to %d resource(s)...\n", policyRulesCount, resourceCount)
		}
	}
	envoyRequests := make([]*authv3.CheckRequest, 0, len(c.EnvoyPayloadPaths))
	for _, path := range c.EnvoyPayloadPaths {
		request, err := processor.LoadEnvoyRequests(path)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("failed to parse envoy payload from %s: %w", path, err)
		}
		envoyRequests = append(envoyRequests, request)
	}
	httpRequests := make([]*authzhttp.CheckRequest, 0, len(c.HTTPPayloadPaths))
	for _, path := range c.HTTPPayloadPaths {
		request, err := processor.LoadHTTPRequests(path)
		if err != nil {
			return nil, nil, skippedInvalidPolicies, nil, fmt.Errorf("failed to parse HTTP payload from %s: %w", path, err)
		}
		httpRequests = append(httpRequests, request)
	}

	// Create output files only after loading inputs to avoid truncating existing files on load errors.
	mutateLogPathIsDir, err := c.getMutateLogPathIsDir()
	if err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}
	if err := c.cleanPreviousContent(mutateLogPathIsDir); err != nil {
		return nil, nil, skippedInvalidPolicies, nil, err
	}

	result, err := evaluator.Evaluate(ctx, policySet, evaluator.Input{
		Resources:          resources,
		JSONPayloads:       jsonPayloads,
		HTTPRequests:       httpRequests,
		EnvoyRequests:      envoyRequests,
		TargetResources:    targetResources,
		ParameterResources: parameterResources,
		Exceptions:         celExceptions,
		Client:             dClient,
		Store:              &store,
		Variables:          variables,
		UserInfo:           userInfo,
		NamespaceResolver:  processor.GetNamespaceResolver(variables, dClient, c.Cluster),
	}, evaluator.Options{
		Out:                out,
		Cluster:            c.Cluster,
		ContinueOnFail:     c.ContinueOnFail,
		Explain:            c.Explain,
		RegistryAccess:     c.RegistryAccess,
		PolicyReport:       c.PolicyReport,
		Stdin:              c.Stdin,
		AuditWarn:          c.AuditWarn,
		PrintPatchResource: true,
		ContextPath:        c.ContextPath,
		CrdPaths:           c.CrdPaths,
		MutateLogPath:      c.MutateLogPath,
		MutateLogPathIsDir: mutateLogPathIsDir,
	})
	return result.Counts, result.Resources, skippedInvalidPolicies, result.Responses, err
}

func (c *ApplyCommandConfig) getMutateLogPathIsDir() (bool, error) {
	if c.OutputDir != "" || c.OutputFile != "" {
		isDir := c.OutputDir != ""
		c.MutateLogPath = c.OutputFile
		if isDir {
			c.MutateLogPath = c.OutputDir
		}
		if info, err := os.Stat(c.MutateLogPath); err == nil && info.IsDir() != isDir {
			return false, fmt.Errorf("output destination %q has the wrong type", c.MutateLogPath)
		}
		return isDir, createFileOrFolder(c.MutateLogPath, isDir)
	}
	mutateLogPathIsDir, err := checkMutateLogPath(c.MutateLogPath)
	if err != nil {
		return false, fmt.Errorf("failed to create file/folder (%w)", err)
	}
	return mutateLogPathIsDir, nil
}

func (c *ApplyCommandConfig) loadResources(out io.Writer, paths []string, policies []engineapi.GenericPolicy, dClient dclient.Interface) ([]*unstructured.Unstructured, []*unstructured.Unstructured, error) {
	resourceOptions := loader.ResourceOptions{
		Namespace:       c.Namespace,
		Concurrency:     c.Concurrent,
		BatchSize:       c.BatchSize,
		ContinueOnError: c.ContinueOnError,
		Timeout:         5 * time.Minute,
	}
	resources, err := common.GetResourceAccordingToResourcePath(out, nil, paths, c.Cluster, policies, dClient, c.Namespace, c.PolicyReport, c.ClusterWideResources, "", resourceOptions, c.ShowPerformance)
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

			object, ok := payload.(map[string]interface{})
			if !ok {
				return nil, nil, fmt.Errorf("JSON payload %s must be a top-level object", path)
			}
			jsonPayloads = append(jsonPayloads, &unstructured.Unstructured{Object: object})
		}
	}
	return resources, jsonPayloads, nil
}

func (c *ApplyCommandConfig) loadPolicies(ctx context.Context, out io.Writer) (*evaluator.PolicySet, error) {
	set := &evaluator.PolicySet{}
	for _, path := range c.PolicyPaths {
		if source.IsOCI(path) {
			tmpDir, cleanup, err := pull.ToTempDir(ctx, source.StripOCIPrefix(path), pull.NewKeychain())
			if err != nil {
				return nil, fmt.Errorf("failed to pull OCI bundle (%w)", err)
			}
			defer cleanup()
			path = tmpDir
		}
		isGit := source.IsGit(path)
		if isGit {
			gitSourceURL, err := url.Parse(path)
			if err != nil {
				return nil, fmt.Errorf("failed to load policies (%w)", err)
			}
			pathElems := strings.Split(gitSourceURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				err := fmt.Errorf("invalid URL path %s - expected https://<any_git_source_domain>/:owner/:repository/:branch (without --git-branch flag) OR https://<any_git_source_domain>/:owner/:repository/:directory (with --git-branch flag)", gitSourceURL.Path)
				return nil, fmt.Errorf("failed to parse URL (%w)", err)
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
			if _, err := c.cloneRepo(repoURL, fs, c.GitBranch, *auth); err != nil {
				log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", err)
				return nil, fmt.Errorf("failed to clone repository (%w)", err)
			}
			policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, fmt.Errorf("failed to list YAMLs in repository (%w)", err)
			}
			for _, policyYaml := range policyYamls {
				loaderResults, err := policy.Load(fs, "", policyYaml)
				if loaderResults != nil && loaderResults.NonFatalErrors != nil {
					for _, err := range loaderResults.NonFatalErrors {
						log.Log.Error(err.Error, "Non-fatal parsing error for single document")
					}
				}
				if err != nil {
					if pkgdeprecations.IsLegacyPolicyBlockError(err) {
						return nil, err
					}
					continue
				}
				c.recordPolicyWarnings(out, loaderResults.Warnings)
				set.Add(loaderResults)
			}
		} else {
			loaderResults, err := policy.Load(nil, "", path)
			if loaderResults != nil && loaderResults.NonFatalErrors != nil {
				for _, err := range loaderResults.NonFatalErrors {
					log.Log.Error(err.Error, "Non-fatal parsing error for single document")
				}
			}
			if err != nil && pkgdeprecations.IsLegacyPolicyBlockError(err) {
				return nil, err
			}
			if err != nil {
				log.Log.V(3).Info("skipping invalid YAML file", "path", path, "error", err)
			} else {
				c.recordPolicyWarnings(out, loaderResults.Warnings)
				set.Add(loaderResults)
			}
		}
	}
	return set, nil
}

func (c *ApplyCommandConfig) recordPolicyWarnings(out io.Writer, warnings []policy.LoaderWarning) {
	for _, warning := range warnings {
		msg := fmt.Sprintf("Warning: %s: %s", warning.Path, warning.Warning)
		fmt.Fprintln(out, msg)
		c.deprecationWarnings = append(c.deprecationWarnings, msg)
	}
}

// recordGlobalContextEntryDeprecationWarnings warns, once per GlobalContextEntry, about any
// projection still using jmesPath, routed through the same deprecationWarnings accumulator as
// every other CLI deprecation warning so --warnings-as-errors picks it up for free.
func (c *ApplyCommandConfig) recordGlobalContextEntryDeprecationWarnings(out io.Writer, gctxEntries []*kyvernov2.GlobalContextEntry) {
	for _, gce := range gctxEntries {
		var jmesPathFields []string
		path := field.NewPath("spec", "projections")
		for i, p := range gce.Spec.Projections {
			if p.JMESPath != "" {
				jmesPathFields = append(jmesPathFields, path.Index(i).Child("jmesPath").String())
			}
		}
		if len(jmesPathFields) == 0 {
			continue
		}
		msg := fmt.Sprintf("GlobalContextEntry %q: %s", gce.Name, pkgdeprecations.BuildProjectionWarning(jmesPathFields).Message)
		fmt.Fprintln(out, "Warning: "+msg)
		c.deprecationWarnings = append(c.deprecationWarnings, msg)
	}
}

func (c *ApplyCommandConfig) failOnDeprecationWarnings() error {
	if c.warningsAsErrors && len(c.deprecationWarnings) > 0 {
		return fmt.Errorf("found %d deprecation warning(s) and --warnings-as-errors is set", len(c.deprecationWarnings))
	}
	return nil
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
		if err != nil {
			return nil, err
		}
		dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	}
	return dClient, err
}

func (c *ApplyCommandConfig) cleanPreviousContent(mutateLogPathIsDir bool) error {
	// empty the previous contents of the file just in case if the file already existed before with some content(so as to perform overwrites)
	// the truncation of files for the case when mutateLogPath is dir, is handled under pkg/kyverno/apply/common.go
	if !mutateLogPathIsDir && c.MutateLogPath != "" {
		c.MutateLogPath = filepath.Clean(c.MutateLogPath)
		// Necessary for us to include the file via variable as it is part of the CLI.
		f, err := os.OpenFile(c.MutateLogPath, os.O_TRUNC|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return fmt.Errorf("failed to truncate the existing file at %s (%w)", c.MutateLogPath, err)
		}
		f.Close()
	}
	return nil
}

func hasStdinPath(paths []string) bool {
	for _, path := range paths {
		if path == "-" {
			return true
		}
	}
	return false
}

// checkExplainCompatible rejects --explain alongside the flags whose stdout is meant for another
// program (a policy report or generated exceptions to parse, or a mutated resource to pipe into
// kubectl). The trace is human-readable text written to the same stdout, so mixing the two would
// leave that output unparseable. These are the same modes that already suppress the
// "Applying N policy rule(s)" banner.
func (c *ApplyCommandConfig) checkExplainCompatible() error {
	if !c.Explain {
		return nil
	}
	for _, f := range []struct {
		set  bool
		name string
	}{
		{c.PolicyReport, "--policy-report"},
		{c.GenerateExceptions, "--generate-exceptions"},
		{c.Stdin, "--stdin"},
	} {
		if f.set {
			return fmt.Errorf("--explain cannot be used with %s: %s output must stay machine-readable, and the trace would be mixed into it", f.name, f.name)
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
	if hasStdinPath(c.PolicyPaths) && hasStdinPath(c.ResourcePaths) {
		return fmt.Errorf("a stdin pipe can be used for either policies or resources, not both")
	}
	if err := c.checkExplainCompatible(); err != nil {
		return err
	}
	if len(c.ResourcePaths) != 0 && len(c.JSONPaths) != 0 {
		return fmt.Errorf("both resource and json files can not be used together, use one or the other")
	}
	if len(c.ResourcePaths) == 0 && len(c.JSONPaths) == 0 && len(c.HTTPPayloadPaths) == 0 && len(c.EnvoyPayloadPaths) == 0 && !c.Cluster {
		return fmt.Errorf("resource file(s) or cluster required")
	}
	normalized := make([]string, 0, len(c.CrdPaths))
	for _, p := range c.CrdPaths {
		if strings.TrimSpace(p) != "" {
			normalized = append(normalized, p)
		}
	}
	c.CrdPaths = normalized
	if len(c.CrdPaths) != 0 && strings.TrimSpace(c.KubeConfig) != "" {
		return fmt.Errorf("crd-paths and kubeconfig flags are mutually exclusive, please use only one of them")
	}
	return nil
}

type WarnExitCodeError struct {
	ExitCode int
}

func (w WarnExitCodeError) Error() string {
	return fmt.Sprintf("exit as warnExitCode is %d", w.ExitCode)
}

func flattenResources(resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	return resource.FlattenResources(resources)
}

func createFakeClientFromResources(resources, targetResources, parameterResources []*unstructured.Unstructured) (dclient.Interface, error) {
	return evaluator.NewFakeClient(resources, targetResources, parameterResources)
}

func exit(out io.Writer, rc *processor.ResultCounts, warnExitCode int, warnNoPassed bool) error {
	if rc.Fail > 0 {
		return fmt.Errorf("exit as there are policy violations")
	} else if rc.Error > 0 {
		return fmt.Errorf("exit as there are policy errors")
	} else if rc.Warn > 0 && warnExitCode != 0 {
		fmt.Fprintf(out, "exit as warnExitCode is %d", warnExitCode)
		return WarnExitCodeError{
			ExitCode: warnExitCode,
		}
	} else if rc.Pass == 0 && warnNoPassed {
		fmt.Fprintln(out, "exit as no objects satisfied policy")
		return WarnExitCodeError{
			ExitCode: warnExitCode,
		}
	}
	return nil
}
