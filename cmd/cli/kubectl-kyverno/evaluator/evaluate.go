package evaluator

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	authzhttp "github.com/kyverno/kyverno-authz/pkg/cel/libs/authz/http"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	dpolengine "github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// Input is the per-evaluation payload: what is evaluated and the context it is evaluated in.
// Evaluate may mutate JSONPayloads in place (a JSON-mode MutatingPolicy patches its payload, as
// apply always has), so callers must not share an Input across evaluations.
type Input struct {
	// Resources are Kubernetes resources (Kubernetes mode).
	Resources []*unstructured.Unstructured
	// JSONPayloads are arbitrary JSON documents (JSON mode).
	JSONPayloads []*unstructured.Unstructured
	// HTTPRequests are Kyverno Authz HTTP check requests (HTTP mode).
	HTTPRequests []*authzhttp.CheckRequest
	// EnvoyRequests are Envoy ext_authz check requests (Envoy mode).
	EnvoyRequests []*authv3.CheckRequest
	// ParameterResources are VAP/MAP parameter objects.
	ParameterResources []*unstructured.Unstructured
	// Exceptions are the CEL PolicyExceptions applied to this evaluation.
	Exceptions []*policiesv1beta1.PolicyException
	// Client is the (real or fake) cluster client used for lookups; may be nil.
	Client dclient.Interface
	// Store holds CLI mock state (global context, API call mocks). Must not be shared across
	// concurrent evaluations.
	Store *store.Store
	// Variables are values-file variables; may be nil.
	Variables *variables.Variables
	// UserInfo is the admission user info; may be nil.
	UserInfo *kyvernov2.RequestInfo
	// NamespaceResolver resolves namespaces for namespaceSelector matching; derived from
	// Variables and Client when nil.
	NamespaceResolver celengine.NamespaceResolver
	// Operation is the admission operation to simulate (CREATE, UPDATE or DELETE). When empty, the
	// `request.operation` global value from Variables is used, then CREATE.
	Operation string
}

// Options controls evaluation behavior. The zero value evaluates silently, stops on the first
// error, and writes no mutate/generate artifacts to disk.
type Options struct {
	// Out receives human-readable progress (patched resources, --explain traces). Defaults to io.Discard.
	Out                io.Writer
	Cluster            bool
	ContinueOnFail     bool
	Explain            bool
	RegistryAccess     bool
	PolicyReport       bool
	Stdin              bool
	AuditWarn          bool
	PrintPatchResource bool
	ContextPath        string
	CrdPaths           []string
	MutateLogPath      string
	MutateLogPathIsDir bool
}

// Result is the outcome of one evaluation.
type Result struct {
	// Counts aggregates rule results the same way `kyverno apply` reports them.
	Counts *processor.ResultCounts
	// Resources are the evaluated Kubernetes resources.
	Resources []*unstructured.Unstructured
	// Responses are the engine responses, in apply's order: vap/vpol/mpol/map/gpol per resource,
	// then ivpol, dpol (resources, then JSON), HTTP and Envoy.
	Responses []engineapi.EngineResponse
}

// Evaluate runs every policy family in set against in. It is the single evaluation path shared by
// `kyverno apply` and `kyverno serve`. It returns the partial result alongside any error.
//
// Evaluate is not safe for concurrent use: the underlying CLI processors rely on process-global
// state (CRD processor, CEL library context). Callers that evaluate concurrently must serialize.
func Evaluate(ctx context.Context, set *PolicySet, in Input, opts Options) (*Result, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if in.Store == nil {
		in.Store = &store.Store{}
	}
	e := &evaluation{opts: opts}
	nsResolver := in.NamespaceResolver
	if nsResolver == nil {
		nsResolver = processor.GetNamespaceResolver(in.Variables, in.Client, opts.Cluster)
	}
	rc, resources, responses1, err := e.applyPolicies(set, in, nsResolver)
	result := &Result{Counts: rc, Resources: resources, Responses: responses1}
	if err != nil {
		return result, err
	}
	responses4, err := e.applyImageValidatingPolicies(set.ImageValidatingPolicies, in.JSONPayloads, resources, in.Exceptions, nsResolver, in.UserInfo, rc, in.Client, operationOf(in))
	if err != nil {
		result.Responses = responses4
		return result, err
	}
	responses5, err := e.applyDeletingPolicies(set.DeletingPolicies, resources, in.Exceptions, nsResolver, rc, in.Client, "resource")
	if err != nil {
		result.Responses = responses4
		return result, err
	}
	responses6, err := e.applyDeletingPolicies(set.DeletingPolicies, in.JSONPayloads, in.Exceptions, nsResolver, rc, in.Client, "json")
	if err != nil {
		result.Responses = responses4
		return result, err
	}
	authzProcessor := processor.NewAuthzProcessor(rc, in.Client, set.HTTPPolicies, set.EnvoyPolicies)
	httpResponses, err := authzProcessor.ApplyHTTPPolicies(in.HTTPRequests)
	if err != nil {
		result.Responses = responses4
		return result, err
	}
	envoyResponses, err := authzProcessor.ApplyEnvoyPolicies(in.EnvoyRequests)
	if err != nil {
		result.Responses = responses4
		return result, err
	}
	responses := make([]engineapi.EngineResponse, 0, len(responses1)+len(responses4)+len(responses5)+len(responses6)+len(httpResponses)+len(envoyResponses))
	responses = append(responses, responses1...)
	responses = append(responses, responses4...)
	responses = append(responses, responses5...)
	responses = append(responses, responses6...)
	responses = append(responses, httpResponses...)
	responses = append(responses, envoyResponses...)
	result.Responses = responses
	return result, nil
}

func operationOf(in Input) string {
	if in.Operation != "" {
		return in.Operation
	}
	if in.Variables != nil {
		return in.Variables.GlobalOperation()
	}
	return ""
}

type evaluation struct {
	opts Options
}

func (e *evaluation) applyPolicies(set *PolicySet, in Input, nsResolver celengine.NamespaceResolver) (*processor.ResultCounts, []*unstructured.Unstructured, []engineapi.EngineResponse, error) {
	c := e.opts
	out := c.Out
	st := in.Store
	vars := in.Variables
	vaps, vapBindings := set.VAPs, set.VAPBindings
	vpols, gpols, mpols := set.ValidatingPolicies, set.GeneratingPolicies, set.MutatingPolicies
	maps, mapBindings := set.MAPs, set.MAPBindings
	resources, jsonPayloads := in.Resources, in.JSONPayloads
	celExceptions := in.Exceptions
	dClient := in.Client

	if vars != nil {
		vars.SetInStore(st)
	}
	var rc processor.ResultCounts
	var responses []engineapi.EngineResponse

	params := make([]runtime.Object, len(in.ParameterResources))
	for i, p := range in.ParameterResources {
		params[i] = p
	}

	crdPreloaded := false
	crdPreloadFailed := false
	if len(c.CrdPaths) > 0 {
		crdPreloaded = true
		for _, crdPath := range c.CrdPaths {
			if strings.TrimSpace(crdPath) == "" {
				continue
			}
			if err := common.LoadCrdsFromPath(crdPath); err != nil {
				if !c.ContinueOnFail {
					return &rc, resources, responses, err
				}
				log.Log.V(2).Info(fmt.Sprintf("failed to load CRDs from %s: %v", crdPath, err))
				rc.IncrementError(1)
				crdPreloaded = false
				crdPreloadFailed = true
				break
			}
		}
	}

	var restMapper meta.RESTMapper
	var contextProvider libs.Context
	var vpolProvider vpolengine.Provider
	var vpolProviderErr error
	var vpolJSONProvider vpolengine.Provider
	var vpolJSONProviderErr error
	var mpolProvider mpolengine.Provider
	var mpolProviderErr error

	if !crdPreloadFailed && (len(resources) > 0 || len(jsonPayloads) > 0) && (len(vpols) != 0 || len(gpols) != 0 || len(mpols) != 0) {
		var err error
		restMapper, err = utils.GetRESTMapper(dClient)
		if err != nil {
			if !c.ContinueOnFail {
				return &rc, resources, responses, err
			}
			rc.IncrementError(1)
		} else {
			contextProvider, err = processor.NewContextProvider(dClient, restMapper, nil, c.ContextPath, true, !c.Cluster, nil, st.GetHTTPMockIndex())
			if err != nil {
				if !c.ContinueOnFail {
					return &rc, resources, responses, err
				}
				rc.IncrementError(1)
				contextProvider = nil
			} else {
				if len(vpols) != 0 {
					compiler := vpolcompiler.NewCompilerWithTrace(c.Explain)
					var jsonPolicies []policiesv1beta1.ValidatingPolicyLike
					var k8sPolicies []policiesv1beta1.ValidatingPolicyLike
					for i := range vpols {
						pol := vpols[i]
						if pol.GetValidatingPolicySpec().EvaluationMode() == policieskyvernoio.EvaluationModeJSON {
							jsonPolicies = append(jsonPolicies, pol)
						} else {
							k8sPolicies = append(k8sPolicies, pol)
						}
					}
					if len(k8sPolicies) > 0 && len(resources) > 0 {
						vpolProvider, err = vpolengine.NewProvider(compiler, k8sPolicies, celExceptions)
						if err != nil {
							if !c.ContinueOnFail {
								return &rc, resources, responses, err
							}
							vpolProvider = nil
							vpolProviderErr = err
						}
					}
					if len(jsonPolicies) > 0 {
						vpolJSONProvider, err = vpolengine.NewProvider(compiler, jsonPolicies, celExceptions)
						if err != nil {
							if !c.ContinueOnFail {
								return &rc, resources, responses, err
							}
							vpolJSONProvider = nil
							vpolJSONProviderErr = err
						}
					}
				}

				if len(mpols) != 0 {
					compiler := mpolcompiler.NewCompilerWithTrace(c.Explain)
					toCompile := slices.DeleteFunc(slices.Clone(mpols), func(policy policiesv1beta1.MutatingPolicyLike) bool {
						if policy.GetSpec().EvaluationMode() == policieskyvernoio.EvaluationModeJSON {
							return len(jsonPayloads) == 0
						}
						return len(resources) == 0
					})
					mpolProvider, err = mpolengine.NewProvider(compiler, toCompile, celExceptions, contextProvider)
					if err != nil {
						if !c.ContinueOnFail {
							return &rc, resources, responses, err
						}
						mpolProvider = nil
						mpolProviderErr = err
					}
				}
			}
		}
	}

	var namespaceSelectors map[string]variables.Labels
	var subresources []v1alpha1.Subresource
	if vars != nil {
		namespaceSelectors = vars.NamespaceSelectors()
		subresources = vars.Subresources()
	}

	for _, resource := range resources {
		processor := processor.PolicyProcessor{
			NamespaceResolver:                 nsResolver,
			Store:                             st,
			ValidatingAdmissionPolicies:       vaps,
			ValidatingAdmissionPolicyBindings: vapBindings,
			ValidatingPolicies:                vpols,
			GeneratingPolicies:                gpols,
			MutatingPolicies:                  mpols,
			MutatingAdmissionPolicies:         maps,
			MutatingAdmissionPolicyBindings:   mapBindings,
			Resource:                          *resource,
			Operation:                         in.Operation,
			CELExceptions:                     celExceptions,
			MutateLogPath:                     c.MutateLogPath,
			MutateLogPathIsDir:                c.MutateLogPathIsDir,
			ParameterResources:                params,
			Variables:                         vars,
			ContextPath:                       c.ContextPath,
			UserInfo:                          in.UserInfo,
			PolicyReport:                      c.PolicyReport,
			NamespaceSelectorMap:              namespaceSelectors,
			Stdin:                             c.Stdin,
			Rc:                                &rc,
			PrintPatchResource:                c.PrintPatchResource,
			Cluster:                           c.Cluster,
			Client:                            dClient,
			AuditWarn:                         c.AuditWarn,
			Explain:                           c.Explain,
			Subresources:                      subresources,
			Out:                               out,
			CrdPaths:                          c.CrdPaths,
			CrdPreloaded:                      crdPreloaded,
			RESTMapper:                        restMapper,
			ContextProvider:                   contextProvider,
			VpolProvider:                      vpolProvider,
			VpolProviderErr:                   vpolProviderErr,
			VpolJSONProvider:                  vpolJSONProvider,
			VpolJSONProviderErr:               vpolJSONProviderErr,
			MpolProvider:                      mpolProvider,
			MpolProviderErr:                   mpolProviderErr,
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
			NamespaceResolver:                 nsResolver,
			Store:                             st,
			ValidatingAdmissionPolicies:       vaps,
			ValidatingAdmissionPolicyBindings: vapBindings,
			ValidatingPolicies:                vpols,
			MutatingPolicies:                  mpols,
			MutatingAdmissionPolicies:         maps,
			MutatingAdmissionPolicyBindings:   mapBindings,
			JsonPayload:                       *resource,
			Operation:                         in.Operation,
			CELExceptions:                     celExceptions,
			MutateLogPath:                     c.MutateLogPath,
			MutateLogPathIsDir:                c.MutateLogPathIsDir,
			Variables:                         vars,
			ContextPath:                       c.ContextPath,
			UserInfo:                          in.UserInfo,
			PolicyReport:                      c.PolicyReport,
			NamespaceSelectorMap:              namespaceSelectors,
			Stdin:                             c.Stdin,
			Rc:                                &rc,
			PrintPatchResource:                c.PrintPatchResource,
			Cluster:                           c.Cluster,
			Client:                            dClient,
			AuditWarn:                         c.AuditWarn,
			Explain:                           c.Explain,
			Subresources:                      subresources,
			Out:                               out,
			CrdPaths:                          c.CrdPaths,
			CrdPreloaded:                      crdPreloaded,
			RESTMapper:                        restMapper,
			ContextProvider:                   contextProvider,
			VpolProvider:                      vpolProvider,
			VpolProviderErr:                   vpolProviderErr,
			VpolJSONProvider:                  vpolJSONProvider,
			VpolJSONProviderErr:               vpolJSONProviderErr,
			MpolProvider:                      mpolProvider,
			MpolProviderErr:                   mpolProviderErr,
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
		*resource = processor.JsonPayload
	}
	return &rc, resources, responses, nil
}

func (e *evaluation) applyImageValidatingPolicies(
	ivps []policiesv1beta1.ImageValidatingPolicyLike,
	jsonPayloads []*unstructured.Unstructured,
	resources []*unstructured.Unstructured,
	celExceptions []*policiesv1beta1.PolicyException,
	namespaceProvider func(string) *corev1.Namespace,
	userInfo *kyvernov2.RequestInfo,
	rc *processor.ResultCounts,
	dclient dclient.Interface,
	operation string,
) ([]engineapi.EngineResponse, error) {
	c := e.opts
	if len(ivps) == 0 {
		return nil, nil
	}

	var lister corev1listers.SecretLister
	if dclient != nil {
		kubeClient := dclient.GetKubeClient()
		informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)

		stopCh := make(chan struct{})
		// This informer will automatically die at the end of this function and thats ok,
		// we don't care about it past applying image validating policies anyways
		defer close(stopCh)
		secretsInformer := informerFactory.Core().V1().Secrets()
		informerFactory.Start(stopCh)
		informerFactory.WaitForCacheSync(stopCh)

		lister = secretsInformer.Lister()
	}

	restMapper, err := utils.GetRESTMapper(dclient)
	if err != nil {
		return nil, err
	}
	contextProvider, err := processor.NewContextProvider(dclient, restMapper, nil, c.ContextPath, c.RegistryAccess, !c.Cluster, nil, nil)
	if err != nil {
		if !c.ContinueOnFail {
			return nil, err
		}
		log.Log.V(2).Info(fmt.Sprintf("failed to initialize context for image validating policies: %v", err))
		if rc != nil {
			rc.IncrementError(1)
		}
		return nil, nil
	}

	// Compilation captures the CLI library defaults, so initialize them first.
	provider, err := ivpolengine.NewProvider(eval.NewCompilerWithTrace(lister, c.Explain), ivps, celExceptions)
	if err != nil {
		return nil, err
	}

	engine := ivpolengine.NewEngine(
		provider,
		namespaceProvider,
		matching.NewMatcher(),
		lister,
		nil,
		config.NewDefaultConfiguration(false),
	)

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
		op, object, oldObject := processor.AdmissionRequestShape(operation, resource)
		request := celengine.Request(
			contextProvider,
			resource.GroupVersionKind(),
			gvr,
			"",
			resource.GetName(),
			resource.GetNamespace(),
			op,
			user,
			object,
			oldObject,
			false,
			nil,
		)
		engineResponse, err := engine.HandleValidating(context.TODO(), request, nil)
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
			e.printTrace(r.Trace)
			if r.Result.Name() == "" {
				// a traced policy that did not apply, or that a match condition skipped: it is only
				// returned for its trace and has no result, like the untraced run, which omits it
				continue
			}
			resp.PolicyResponse.Rules = []engineapi.RuleResponse{r.Result}
			resp = resp.WithPolicy(engineapi.NewImageValidatingPolicyFromLike(r.Policy))
			rc.AddValidatingPolicyResponse(resp)
			responses = append(responses, resp)
		}
	}

	ivpols := make([]*eval.CompiledImageValidatingPolicy, 0)
	pMap := make(map[string]policiesv1beta1.ImageValidatingPolicyLike)

	for i := range ivps {
		p := ivps[i]
		pMap[p.GetName()] = p
		ivpols = append(ivpols, &eval.CompiledImageValidatingPolicy{Policy: p})
	}

	for _, json := range jsonPayloads {
		result, err := eval.EvaluateWithTrace(context.TODO(), ivpols, json.Object, nil, nil, lister, c.Explain)
		// before the error is handled, so a policy that errored still shows how far it got
		for _, name := range slices.Sorted(maps.Keys(result)) {
			if rslt := result[name]; rslt != nil {
				e.printTrace(rslt.Trace)
			}
		}
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
			if rslt == nil || rslt.Skipped {
				// a match condition excluded the payload, so there is nothing to report
				continue
			}
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
			resp = resp.WithPolicy(engineapi.NewImageValidatingPolicyFromLike(pMap[p]))
			rc.AddValidatingPolicyResponse(resp)
			responses = append(responses, resp)
		}
	}
	return responses, nil
}

// printTrace prints one decision trace. It prints nothing unless Explain is set and the policy
// was traced, so it is safe to call for every response.
func (e *evaluation) printTrace(d *trace.Decision) {
	if !e.opts.Explain || d == nil {
		return
	}
	trace.Render(e.opts.Out, d)
	fmt.Fprintln(e.opts.Out)
}

func (e *evaluation) applyDeletingPolicies(
	dps []policiesv1beta1.DeletingPolicyLike,
	resources []*unstructured.Unstructured,
	celExceptions []*policiesv1beta1.PolicyException,
	namespaceProvider func(string) *corev1.Namespace,
	rc *processor.ResultCounts,
	dclient dclient.Interface,
	payloadType string,
) ([]engineapi.EngineResponse, error) {
	c := e.opts
	out := c.Out
	if len(dps) == 0 || len(resources) == 0 {
		return nil, nil
	}

	restMapper, err := utils.GetRESTMapper(dclient)
	if err != nil {
		return nil, err
	}

	contextProvider, err := processor.NewContextProvider(dclient, restMapper, nil, c.ContextPath, c.RegistryAccess, !c.Cluster, nil, nil)
	if err != nil {
		if !c.ContinueOnFail {
			return nil, err
		}
		log.Log.V(2).Info(fmt.Sprintf("failed to initialize context for deleting policies: %v", err))
		if rc != nil {
			rc.IncrementError(1)
		}
		return nil, nil
	}

	// Compilation captures the CLI library defaults, so initialize them first.
	provider, err := dpolengine.NewProvider(dpolcompiler.NewCompilerWithTrace(c.Explain), dps, celExceptions)
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
			genericPolicy := engineapi.NewDeletingPolicyFromLike(dpol.Policy)
			if genericPolicy == nil {
				return nil, fmt.Errorf("unsupported deleting policy type %T", dpol.Policy)
			}
			policyName := dpol.Policy.GetName()
			resp, err := engine.Handle(context.TODO(), dpol, *resource)
			// before the error and not-matched branches below, so their traces are printed too;
			// Trace is only ever set when the policy was compiled with --explain
			if resp.Trace != nil {
				trace.Render(out, resp.Trace)
				fmt.Fprintln(out)
			}
			if err != nil {
				response := engineapi.NewEngineResponse(*resource, genericPolicy, nil)
				response = response.WithPolicyResponse(engineapi.PolicyResponse{Rules: []engineapi.RuleResponse{
					*engineapi.NewRuleResponse(policyName, engineapi.Deletion, err.Error(), engineapi.RuleStatusError, nil),
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
				if !resp.PolicyMatched {
					// The resource is not selected by the policy's matchConstraints
					// (resourceRules, objectSelector, namespaceSelector). Align with
					// the other CEL policy types (vpol/mpol), which emit no result
					// row for constraint-excluded resources.
					continue
				}
				status = engineapi.RuleStatusFail
				message = fmt.Sprintf("%s did not match", payloadType)
			}

			response := engineapi.NewEngineResponse(*resource, genericPolicy, nil)
			response = response.WithPolicyResponse(engineapi.PolicyResponse{Rules: []engineapi.RuleResponse{
				*engineapi.NewRuleResponse(policyName, engineapi.Deletion, message, status, nil),
			}})

			responses = append(responses, response)

			rc.AddValidatingPolicyResponse(response)
		}
	}

	return responses, nil
}
