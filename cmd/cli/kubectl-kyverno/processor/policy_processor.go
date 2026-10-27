package processor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	mpol "github.com/kyverno/kyverno/pkg/background/mpol"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	celutils "github.com/kyverno/sdk/extensions/cel/utils"
	"go.yaml.in/yaml/v3"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
	"k8s.io/client-go/openapi"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

type PolicyProcessor struct {
	Store                             *store.Store
	ValidatingAdmissionPolicies       []admissionregistrationv1.ValidatingAdmissionPolicy
	ValidatingAdmissionPolicyBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	MutatingAdmissionPolicies         []admissionregistrationv1beta1.MutatingAdmissionPolicy
	MutatingAdmissionPolicyBindings   []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	ValidatingPolicies                []policiesv1beta1.ValidatingPolicyLike
	EnvoyPolicies                     []*policiesv1beta1.ValidatingPolicy
	HTTPPolicies                      []*policiesv1beta1.ValidatingPolicy
	GeneratingPolicies                []policiesv1beta1.GeneratingPolicyLike
	MutatingPolicies                  []policiesv1beta1.MutatingPolicyLike
	TargetResources                   []*unstructured.Unstructured
	Resource                          unstructured.Unstructured
	JsonPayload                       unstructured.Unstructured
	// Operation is the admission operation to simulate (CREATE, UPDATE or DELETE).
	// When empty, the `request.operation` global value from the values file is
	// honored, defaulting to CREATE.
	Operation          string
	CELExceptions      []*policiesv1beta1.PolicyException
	MutateLogPath      string
	MutateLogPathIsDir bool
	Variables          *variables.Variables
	ParameterResources []runtime.Object
	// TODO
	ContextFs            billy.Filesystem
	ContextPath          string
	GlobalContextEntries map[string]interface{}
	Cluster              bool
	UserInfo             *kyvernov2.RequestInfo
	PolicyReport         bool
	NamespaceSelectorMap map[string]map[string]string
	Stdin                bool
	Rc                   *ResultCounts
	PrintPatchResource   bool
	Client               dclient.Interface
	AuditWarn            bool
	Subresources         []v1alpha1.Subresource
	Out                  io.Writer
	CrdPaths             []string
	CrdPreloaded         bool
	ConfigMapResolver    engineapi.ConfigmapResolver
	RESTMapper           meta.RESTMapper
	ContextProvider      libs.Context
	VpolProvider         vpolengine.Provider
	VpolProviderErr      error
	VpolJSONProvider     vpolengine.Provider
	VpolJSONProviderErr  error
	MpolProvider         mpolengine.Provider
	MpolProviderErr      error
	// Explain compiles validating and mutating policies with tracing on and prints, for each
	// policy, how it arrived at its result (scope, match conditions, variables, the validations
	// or mutations it ran, and verdict).
	Explain bool
}

func (p *PolicyProcessor) ApplyPoliciesOnResource() ([]engineapi.EngineResponse, error) {
	resource := p.Resource
	if !p.CrdPreloaded && len(p.CrdPaths) > 0 {
		if err := p.loadCrds(); err != nil {
			return nil, err
		}
	}
	gvk, subresource := resource.GroupVersionKind(), ""
	resourceKind := resource.GetKind()
	resourceName := resource.GetName()
	resourceNamespace := resource.GetNamespace()
	// If --cluster flag is not set, then we need to find the top level resource GVK and subresource
	if !p.Cluster {
		for _, s := range p.Subresources {
			subgvk := schema.GroupVersionKind{
				Group:   s.Subresource.Group,
				Version: s.Subresource.Version,
				Kind:    s.Subresource.Kind,
			}
			if gvk == subgvk {
				gvk = schema.GroupVersionKind{
					Group:   s.ParentResource.Group,
					Version: s.ParentResource.Version,
					Kind:    s.ParentResource.Kind,
				}
				parts := strings.Split(s.Subresource.Name, "/")
				subresource = parts[1]
			}
		}
	}
	resPath := fmt.Sprintf("%s/%s/%s", resourceNamespace, resourceKind, resourceName)
	var responses []engineapi.EngineResponse

	restMapper := p.RESTMapper
	if restMapper == nil {
		var err error
		restMapper, err = utils.GetRESTMapper(p.Client)
		if err != nil {
			return nil, err
		}
	}
	// Mutate Admission Policies
	if len(p.MutatingAdmissionPolicies) != 0 {
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
		} else {
			var user authenticationv1.UserInfo
			if p.UserInfo != nil {
				user = p.UserInfo.AdmissionUserInfo
			}
			gvr := mapping.Resource
			for _, mapPolicy := range p.MutatingAdmissionPolicies {
				data := engineapi.NewMutatingAdmissionPolicyData(&mapPolicy)
				for _, b := range p.MutatingAdmissionPolicyBindings {
					if b.Spec.PolicyName == mapPolicy.Name {
						data.AddBinding(b)
					}
				}
				for _, param := range p.ParameterResources {
					data.AddParam(param)
				}
				mutateResponse, err := admissionpolicy.Mutate(data, resource, gvk, gvr, p.NamespaceSelectorMap, p.Client, &user, !p.Cluster, false)
				if err != nil {
					log.Log.Error(err, "failed to apply MAP", "policy", mapPolicy.Name)
					continue
				}
				if mutateResponse.IsEmpty() {
					continue
				}
				// its fine to just error here because this function just logs the error
				if err := p.processMutateEngineResponse(mutateResponse, resPath); err != nil {
					log.Log.Error(err, "failed to log MAP mutation")
				}
				resource = mutateResponse.PatchedResource
				responses = append(responses, mutateResponse)
			}
		}
	}
	// MutatingPolicies
	if len(p.MutatingPolicies) != 0 {
		compiler := mpolcompiler.NewCompilerWithTrace(p.Explain)
		contextProvider := p.ContextProvider
		if contextProvider == nil {
			var err error
			contextProvider, err = NewContextProvider(p.Client, restMapper, p.ContextFs, p.ContextPath, true, !p.Cluster, p.GlobalContextEntries, p.Store.GetHTTPMockIndex())
			if err != nil {
				return nil, err
			}
		} else {
			libs.LibraryContext = contextProvider
		}

		if p.MpolProviderErr != nil {
			return nil, p.MpolProviderErr
		}
		provider := p.MpolProvider
		if provider == nil {
			var err error
			provider, err = mpolengine.NewProvider(compiler, p.MutatingPolicies, p.CELExceptions, contextProvider)
			if err != nil {
				return nil, err
			}
		}
		if resource.Object != nil {
			tcm := mpolcompiler.NewStaticTypeConverterManager(p.openAPI())
			nsResolver := p.getNamespaceResolver()
			eng := mpolengine.NewEngine(provider, nsResolver, matching.NewMatcher(), tcm, contextProvider)
			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				if !p.Cluster {
					mapping = &meta.RESTMapping{
						Resource: schema.GroupVersionResource{
							Group:   gvk.Group,
							Version: gvk.Version,
						},
					}

					newR, err := p.resolveResource(gvk.Kind)
					if err != nil {
						return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
					}
					mapping.Resource.Resource = newR
				} else {
					return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
				}
			}
			gvr := mapping.Resource
			var user authenticationv1.UserInfo
			if p.UserInfo != nil {
				user = p.UserInfo.AdmissionUserInfo
			}
			// create engine request
			operation, object, oldObject := AdmissionRequestShape(p.resolveOperation(), &resource)
			request := celengine.Request(
				contextProvider,
				gvk,
				gvr,
				subresource,
				resource.GetName(),
				resource.GetNamespace(),
				operation,
				user,
				object,
				oldObject,
				false,
				nil,
			)
			// object and request point at resource, which the inline loop below overwrites with each patched
			// result. The mutate-existing pass needs the trigger as it was admitted, so snapshot it first.
			admitted := newAdmittedTrigger(contextProvider, resource.DeepCopy(), gvk, gvr, subresource, p.resolveOperation(), user)
			// Skip inline mutation for policies with active target constraints, as admission does.
			reps, err := eng.Handle(context.TODO(), request, mpolengine.NoTargetMatchConstraintPolicy())
			if err != nil {
				return nil, fmt.Errorf("failed to apply mutating policies on resource %s (%w)", resource.GetName(), err)
			}
			p.explainMutations(reps.Policies)
			for _, r := range reps.Policies {
				if len(r.Rules) == 0 && hasSelector(r.Policy.GetSpec().MatchConstraints) {
					continue
				}
				patched := *reps.Resource
				if reps.PatchedResource != nil {
					patched = *reps.PatchedResource
				}

				response := engineapi.EngineResponse{
					Resource:        *reps.Resource,
					PatchedResource: patched,
					PolicyResponse: engineapi.PolicyResponse{
						Rules: r.Rules,
					},
				}
				response = response.WithPolicy(engineapi.NewMutatingPolicyFromLike(r.Policy))

				// processMutateEngineResponse counts the response, so it is not counted here as well.
				// A response with no rules, such as the autogen copy of a Pod policy evaluated against a
				// Pod, mutated nothing, so it is not printed or written to the output file.
				if len(r.Rules) > 0 {
					if err := p.processMutateEngineResponse(response, resPath); err != nil {
						return responses, fmt.Errorf("failed to print mutated result (%w)", err)
					}
				}

				responses = append(responses, response)
				resource = response.PatchedResource
			}
			// mutateExisting MutatingPolicies - process target resources. Like the admission webhook, only
			// policies whose trigger rules (matchConstraints, matchConditions) match this resource may
			// mutate targets; the target engine below has no matcher and never checks the trigger itself.
			// When no target is supplied or no policy is triggered there is nothing to evaluate.
			// Which policies are triggered is decided on the admitted trigger, as the admission webhook does,
			// and the matchConditions of the policies that select their targets with a CEL expression are
			// checked again on the trigger as it is now stored, as the background controller does before it
			// evaluates that expression.
			var triggeredPolicies []string
			var targetContext libs.Context
			if len(p.TargetResources) > 0 {
				// The targets exist before the trigger is admitted, so they are registered before the trigger
				// checks: without a cluster that makes them visible to every CEL resource.get()/resource.list()
				// of the policies, and the CEL target expression sees them in cluster mode too.
				targetContext = targetResourceContext(contextProvider, p.TargetResources, restMapper)
				triggeredPolicies = triggeredMutateExistingPolicies(context.TODO(), provider, admitted.attr, &admitted.request.Request, nsResolver(resource.GetNamespace()), contextProvider)
				storedTrigger := admission.NewAttributesRecord(&resource, admitted.oldObject, gvk, resource.GetNamespace(), resource.GetName(), gvr, subresource, admitted.operation, nil, false, admissionpolicy.NewUser(user))
				triggeredPolicies = conditionsStillMatch(context.TODO(), provider, triggeredPolicies, storedTrigger, &admitted.request.Request, nsResolver(resource.GetNamespace()), contextProvider)
			}
			if len(triggeredPolicies) > 0 {
				// Create engine with nil matcher — targets are filtered by targetMatchConstraints
				// (via label selectors and CEL expressions) rather than by MatchConstraints which matches triggers
				mutExistEng := mpolengine.NewEngine(provider, nsResolver, nil, tcm, targetEngineContext(p.Cluster, contextProvider, targetContext))
				targetMatcher := matching.NewMatcher()
				celTargets, err := discoverCELTargets(provider, targetContext, &resource, triggeredPolicies)
				if err != nil {
					return nil, fmt.Errorf("failed to discover CEL targets: %w", err)
				}
				for _, target := range p.TargetResources {
					targetGVK := target.GroupVersionKind()
					targetMapping, err := restMapper.RESTMapping(targetGVK.GroupKind(), targetGVK.Version)
					if err != nil {
						return nil, fmt.Errorf("failed to map target gvk to gvr %s: %w", targetGVK, err)
					}
					targetGVR := targetMapping.Resource
					attr := admission.NewAttributesRecord(
						target,
						nil,
						targetGVK,
						target.GetNamespace(),
						target.GetName(),
						targetGVR,
						"",
						admission.Operation(""),
						nil,
						false,
						nil,
					)
					// a target namespaceSelector is matched against the labels of the target's namespace
					var targetNamespace *corev1.Namespace
					if ns := target.GetNamespace(); ns != "" {
						targetNamespace = nsResolver(ns)
					}
					// targets are evaluated with the admission request of the admitted trigger, as the background controller does
					evalResponse, err := mutExistEng.Evaluate(context.TODO(), attr, admitted.request.Request, mpolengine.And(policyKeyIn(triggeredPolicies), targetMatchPredicate(targetMatcher, attr, targetNamespace, celTargets)))
					if err != nil {
						return nil, fmt.Errorf("failed to evaluate mutateExisting policies on target %s (%w)", target.GetName(), err)
					}
					p.explainMutations(evalResponse.Policies)
					for _, r := range evalResponse.Policies {
						if len(r.Rules) == 0 {
							continue
						}
						patched := *evalResponse.Resource
						if evalResponse.PatchedResource != nil {
							patched = *evalResponse.PatchedResource
						}
						rules := make([]engineapi.RuleResponse, 0, len(r.Rules))
						for _, rule := range r.Rules {
							props := rule.Properties()
							if props == nil {
								props = make(map[string]string)
							} else {
								newProps := make(map[string]string, len(props)+1)
								for k, v := range props {
									newProps[k] = v
								}
								props = newProps
							}
							props["kyverno.io/target"] = "true"
							rWithProp := rule.WithProperties(props)
							if rule.Status() == engineapi.RuleStatusPass {
								rules = append(rules, *rWithProp.WithPatchedTarget(&patched, metav1.GroupVersionResource(targetGVR), ""))
							} else {
								rules = append(rules, *rWithProp)
							}
						}
						resp := engineapi.EngineResponse{
							Resource: *target,
							PolicyResponse: engineapi.PolicyResponse{
								Rules: rules,
							},
						}
						resp = resp.WithPolicy(engineapi.NewMutatingPolicyFromLike(r.Policy))
						if err := p.processMutateExistingEngineResponse(resp, resPath); err != nil {
							return responses, fmt.Errorf("failed to print mutated target result (%w)", err)
						}
						responses = append(responses, resp)
					}
				}
			}
		}
	}
	// validating admission policies
	vapResponses := make([]engineapi.EngineResponse, 0, len(p.ValidatingAdmissionPolicies))
	if len(p.ValidatingAdmissionPolicies) != 0 {
		// map gvk to gvr
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
		}
		gvr := mapping.Resource
		var user authenticationv1.UserInfo
		if p.UserInfo != nil {
			user = p.UserInfo.AdmissionUserInfo
		}
		for _, policy := range p.ValidatingAdmissionPolicies {
			policyData := engineapi.NewValidatingAdmissionPolicyData(&policy)
			for _, binding := range p.ValidatingAdmissionPolicyBindings {
				if binding.Spec.PolicyName == policy.Name {
					policyData.AddBinding(binding)
				}
			}
			for _, param := range p.ParameterResources {
				policyData.AddParam(param)
			}
			validateResponse, _ := admissionpolicy.Validate(policyData, resource, gvk, gvr, p.NamespaceSelectorMap, p.Client, &user, !p.Cluster)
			vapResponses = append(vapResponses, validateResponse)
			p.Rc.addValidatingAdmissionResponse(validateResponse)
		}
	}
	// validating policies
	if len(p.ValidatingPolicies) != 0 {
		ctx := context.TODO()
		compiler := vpolcompiler.NewCompilerWithTrace(p.Explain)
		// Separate policies by evaluation mode to route them correctly.
		// JSON-mode policies evaluate against raw JSON and must not go through the
		// Kubernetes admission path (which requires GVK/GVR and admission attributes).
		// Kubernetes-mode policies (the default) require admission attributes and must
		// not be sent through the JSON path (which would cause a nil pointer dereference).
		jsonPolicies := make([]policiesv1beta1.ValidatingPolicyLike, 0)
		k8sPolicies := make([]policiesv1beta1.ValidatingPolicyLike, 0)
		for i := range p.ValidatingPolicies {
			pol := p.ValidatingPolicies[i]
			if pol.GetValidatingPolicySpec().EvaluationMode() == policieskyvernoio.EvaluationModeJSON {
				jsonPolicies = append(jsonPolicies, pol)
			} else {
				k8sPolicies = append(k8sPolicies, pol)
			}
		}
		contextProvider := p.ContextProvider
		if contextProvider == nil {
			var err error
			contextProvider, err = NewContextProvider(p.Client, restMapper, p.ContextFs, p.ContextPath, true, !p.Cluster, p.GlobalContextEntries, p.Store.GetHTTPMockIndex())
			if err != nil {
				return nil, err
			}
		} else {
			libs.LibraryContext = contextProvider
		}
		if resource.Object != nil {
			// Evaluate Kubernetes-mode policies via the admission path
			if len(k8sPolicies) > 0 {
				if p.VpolProviderErr != nil {
					return nil, p.VpolProviderErr
				}
				provider := p.VpolProvider
				if provider == nil {
					var err error
					provider, err = vpolengine.NewProvider(compiler, k8sPolicies, p.CELExceptions)
					if err != nil {
						return nil, err
					}
				}
				eng := vpolengine.NewEngine(provider, p.getNamespaceResolver(), matching.NewMatcher())
				// map gvk to gvr
				mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
				if err != nil {
					if !p.Cluster {
						mapping = &meta.RESTMapping{
							Resource: schema.GroupVersionResource{
								Group:   gvk.Group,
								Version: gvk.Version,
							},
						}

						newR, err := p.resolveResource(gvk.Kind)
						if err != nil {
							return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
						}
						mapping.Resource.Resource = newR
					} else {
						return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
					}
				}

				gvr := mapping.Resource
				var user authenticationv1.UserInfo
				if p.UserInfo != nil {
					user = p.UserInfo.AdmissionUserInfo
				}
				// create engine request
				operation, object, oldObject := AdmissionRequestShape(p.resolveOperation(), &resource)
				request := celengine.Request(
					contextProvider,
					gvk,
					gvr,
					subresource,
					resource.GetName(),
					resource.GetNamespace(),
					operation,
					user,
					object,
					oldObject,
					false,
					nil,
				)
				reps, err := eng.Handle(ctx, request, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to apply validating policies on resource %s (%w)", resource.GetName(), err)
				}
				p.explain(reps.Policies)
				for _, r := range reps.Policies {
					if len(r.Rules) == 0 && hasSelector(r.Policy.GetSpec().MatchConstraints) {
						continue
					}
					response := engineapi.EngineResponse{
						Resource: *reps.Resource,
						PolicyResponse: engineapi.PolicyResponse{
							Rules: r.Rules,
						},
					}
					response = response.WithPolicy(engineapi.NewValidatingPolicyFromLike(r.Policy))
					p.Rc.AddValidatingPolicyResponse(response)
					responses = append(responses, response)
				}
			}
			// Also evaluate JSON-mode policies against the K8s resource as raw JSON
			if len(jsonPolicies) > 0 {
				if p.VpolJSONProviderErr != nil {
					return nil, p.VpolJSONProviderErr
				}
				provider := p.VpolJSONProvider
				if provider == nil {
					var err error
					provider, err = vpolengine.NewProvider(compiler, jsonPolicies, p.CELExceptions)
					if err != nil {
						return nil, err
					}
				}
				eng := vpolengine.NewEngine(provider, nil, nil)
				request := celengine.RequestFromJSON(contextProvider, &resource)
				reps, err := eng.Handle(ctx, request, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to apply JSON-mode validating policies on resource %s (%w)", resource.GetName(), err)
				}
				p.explain(reps.Policies)
				for _, r := range reps.Policies {
					if len(r.Rules) == 0 {
						continue
					}
					response := engineapi.EngineResponse{
						Resource: *reps.Resource,
						PolicyResponse: engineapi.PolicyResponse{
							Rules: r.Rules,
						},
					}
					response = response.WithPolicy(engineapi.NewValidatingPolicyFromLike(r.Policy))
					p.Rc.AddValidatingPolicyResponse(response)
					responses = append(responses, response)
				}
			}
		}
		if p.JsonPayload.Object != nil {
			if len(k8sPolicies) > 0 {
				log.Log.V(1).Info("skipping Kubernetes-mode validating policies for JSON payload (set spec.evaluation.mode to JSON for non-Kubernetes payloads)",
					"skippedPolicies", len(k8sPolicies))
			}
			if len(jsonPolicies) > 0 {
				if p.VpolJSONProviderErr != nil {
					return nil, p.VpolJSONProviderErr
				}
				provider := p.VpolJSONProvider
				if provider == nil {
					var err error
					provider, err = vpolengine.NewProvider(compiler, jsonPolicies, p.CELExceptions)
					if err != nil {
						return nil, err
					}
				}
				eng := vpolengine.NewEngine(provider, nil, nil)
				request := celengine.RequestFromJSON(contextProvider, &unstructured.Unstructured{Object: p.JsonPayload.Object})
				reps, err := eng.Handle(ctx, request, nil)
				if err != nil {
					return nil, err
				}
				p.explain(reps.Policies)
				for _, r := range reps.Policies {
					response := engineapi.EngineResponse{
						Resource: *reps.Resource,
						PolicyResponse: engineapi.PolicyResponse{
							Rules: r.Rules,
						},
					}
					response = response.WithPolicy(engineapi.NewValidatingPolicyFromLike(r.Policy))
					p.Rc.AddValidatingPolicyResponse(response)
					responses = append(responses, response)
				}
			}
		}
	}
	// generating policies
	if len(p.GeneratingPolicies) != 0 {
		// initialize the context provider before compiling to make it globally available
		contextProvider := p.ContextProvider
		if contextProvider == nil {
			var err error
			contextProvider, err = NewContextProvider(p.Client, restMapper, p.ContextFs, p.ContextPath, true, !p.Cluster, p.GlobalContextEntries, p.Store.GetHTTPMockIndex())
			if err != nil {
				return nil, err
			}
		} else {
			libs.LibraryContext = contextProvider
		}

		compiler := gpolcompiler.NewCompilerWithTrace(p.Explain)
		compiledPolicies := make([]gpolengine.Policy, 0, len(p.GeneratingPolicies))
		for _, pol := range p.GeneratingPolicies {
			compiled, errs := compiler.Compile(pol, p.CELExceptions)
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to compile policy %s (%w)", pol.GetName(), errs.ToAggregate())
			}
			compiledPolicies = append(compiledPolicies, gpolengine.Policy{
				Policy:         engineapi.NewGeneratingPolicyFromLike(pol).AsGeneratingPolicy(),
				CompiledPolicy: compiled,
			})
		}
		if resource.Object != nil {
			engine := gpolengine.NewEngine(p.getNamespaceResolver(), matching.NewMatcher())
			// map gvk to gvr
			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				if !p.Cluster {
					mapping = &meta.RESTMapping{
						Resource: schema.GroupVersionResource{
							Group:   gvk.Group,
							Version: gvk.Version,
						},
					}

					newR, err := p.resolveResource(gvk.Kind)
					if err != nil {
						return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
					}
					mapping.Resource.Resource = newR
				} else {
					return nil, fmt.Errorf("failed to map gvk to gvr %s: %w", gvk, err)
				}
			}
			gvr := mapping.Resource
			var user authenticationv1.UserInfo
			if p.UserInfo != nil {
				user = p.UserInfo.AdmissionUserInfo
			}
			// create engine request
			operation, object, oldObject := AdmissionRequestShape(p.resolveOperation(), &resource)
			request := celengine.Request(
				contextProvider,
				gvk,
				gvr,
				subresource,
				resource.GetName(),
				resource.GetNamespace(),
				operation,
				user,
				object,
				oldObject,
				false,
				nil,
			)
			for _, policy := range compiledPolicies {
				engineResponse, err := engine.Handle(context.TODO(), request, policy, false)
				if err != nil {
					return nil, err
				}
				for _, res := range engineResponse.Policies {
					// before the no-result branch below, so a skipped policy's trace is printed too
					if p.Explain && res.Trace != nil {
						// the generate output of the previous resource ends without a newline
						fmt.Fprintln(p.Out)
						trace.Render(p.Out, res.Trace)
						fmt.Fprintln(p.Out)
					}
					if res.Result == nil {
						generateResponse := engineapi.EngineResponse{
							Resource: *engineResponse.Trigger,
						}
						generateResponse = generateResponse.WithPolicy(engineapi.NewGeneratingPolicyFromLike(res.Policy))
						p.Rc.addGenerateResponse(generateResponse)
						responses = append(responses, generateResponse)
						continue
					}
					generateResponse := engineapi.EngineResponse{
						Resource: *engineResponse.Trigger,
						PolicyResponse: engineapi.PolicyResponse{
							Rules: []engineapi.RuleResponse{*res.Result},
						},
					}
					generateResponse = generateResponse.WithPolicy(engineapi.NewGeneratingPolicyFromLike(res.Policy))
					if err := p.processGenerateResponse(generateResponse, resPath); err != nil {
						return responses, err
					}
					p.Rc.addGenerateResponse(generateResponse)
					responses = append(responses, generateResponse)
				}
			}
		}
	}
	responses = append(responses, vapResponses...)
	return responses, nil
}

func (p *PolicyProcessor) processGenerateResponse(response engineapi.EngineResponse, resourcePath string) error {
	generatedResources := []*unstructured.Unstructured{}
	for _, rule := range response.PolicyResponse.Rules {
		gen := rule.GeneratedResources()
		generatedResources = append(generatedResources, gen...)
	}
	for _, r := range generatedResources {
		err := p.printOutput(r.Object, response, resourcePath, true)
		if err != nil {
			return fmt.Errorf("failed to print generate result (%w)", err)
		}
		fmt.Fprintf(p.Out, "\n\nGenerate:\nGeneration completed successfully.")
	}
	return nil
}

func (p *PolicyProcessor) processMutateEngineResponse(response engineapi.EngineResponse, resourcePath string) error {
	p.Rc.addMutateResponse(response)
	err := p.printOutput(response.PatchedResource.Object, response, resourcePath, false)
	if err != nil {
		return fmt.Errorf("failed to print mutated result (%w)", err)
	}
	fmt.Fprintf(p.Out, "\n\nMutation:\nMutation has been applied successfully.")
	return nil
}

// processMutateExistingEngineResponse records and prints a response produced for a mutateExisting target.
// Only the patched targets are printed, the trigger itself is left untouched.
func (p *PolicyProcessor) processMutateExistingEngineResponse(response engineapi.EngineResponse, resourcePath string) error {
	if !p.Rc.addMutateResponse(response) {
		return nil
	}
	return p.printOutput(nil, response, resourcePath, false)
}

// printOutput prints the given resource followed by any patched targets found in the response.
// A nil resource prints only the patched targets.
func (p *PolicyProcessor) printOutput(resource interface{}, response engineapi.EngineResponse, resourcePath string, isGenerate bool) error {
	var yamlEncodedResource []byte
	if resource != nil {
		var err error
		yamlEncodedResource, err = yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("failed to marshal (%w)", err)
		}
	}
	printResource := resource != nil

	var yamlEncodedTargetResources [][]byte
	for _, ruleResponese := range response.PolicyResponse.Rules {
		patchedTarget, _, _ := ruleResponese.PatchedTarget()

		if patchedTarget != nil {
			yamlEncodedResource, err := yaml.Marshal(patchedTarget.Object)
			if err != nil {
				return fmt.Errorf("failed to marshal (%w)", err)
			}

			yamlEncodedResource = append(yamlEncodedResource, []byte("\n---\n")...)
			yamlEncodedTargetResources = append(yamlEncodedTargetResources, yamlEncodedResource)
		}
	}

	if p.MutateLogPath == "" {
		resource := string(yamlEncodedResource) + string("\n---")
		if !printResource || len(strings.TrimSpace(resource)) > 0 {
			if !p.Stdin {
				fmt.Fprintf(p.Out, "\npolicy %s applied to %s:", response.Policy().GetName(), resourcePath)
			}
			if printResource {
				fmt.Fprint(p.Out, "\n"+resource+"\n")
			} else {
				fmt.Fprint(p.Out, "\n")
			}
			if len(yamlEncodedTargetResources) > 0 {
				fmt.Fprintf(p.Out, "patched targets: \n")
				for _, patchedTarget := range yamlEncodedTargetResources {
					fmt.Fprint(p.Out, "\n"+string(patchedTarget)+"\n")
				}
			}
		}
		return nil
	}

	var file *os.File
	var err error
	mutateLogPath := filepath.Clean(p.MutateLogPath)
	filename := p.Resource.GetName() + "-mutated"
	if isGenerate {
		filename = response.Policy().GetName() + "-generated"
	}

	if p.MutateLogPathIsDir {
		file, err = os.OpenFile(filepath.Join(mutateLogPath, filename+".yaml"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) // #nosec G304
		if err != nil {
			return err
		}
	} else {
		// truncation for the case when mutateLogPath is a file (not a directory) is handled under pkg/kyverno/apply/test_command.go
		file, err = os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return err
		}
	}
	if printResource {
		if _, err := file.Write([]byte(string(yamlEncodedResource) + "\n---\n\n")); err != nil {
			return err
		}
	}

	for _, patchedTarget := range yamlEncodedTargetResources {
		if _, err := file.Write(patchedTarget); err != nil {
			return err
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}

func (p *PolicyProcessor) openAPI() openapi.Client {
	clients := make([]openapi.Client, 0)

	if p.Cluster {
		// In CLI test mode the discovery client is a fake that panics on
		// OpenAPIV3(), so we recover and skip it gracefully.
		if client := p.tryClusterOpenAPI(); client != nil {
			clients = append(clients, client)
		}
	}

	addedCrdClient := false
	if len(p.CrdPaths) > 0 {
		visitedDirs := make(map[string]struct{})
		for _, crdPath := range p.CrdPaths {
			if strings.TrimSpace(crdPath) == "" {
				continue
			}
			absPath := getAbsPath(crdPath)
			if _, seen := visitedDirs[absPath]; !seen {
				diskCrds := os.DirFS(absPath)
				clients = append(clients, openapiclient.NewLocalCRDFiles(diskCrds))
				visitedDirs[absPath] = struct{}{}
				addedCrdClient = true
			}
		}
	}
	if !addedCrdClient {
		if crds, err := data.Crds(); err == nil {
			clients = append(clients, openapiclient.NewLocalSchemaFiles(crds))
		}
	}

	clients = append(clients, openapiclient.NewHardcodedBuiltins("1.32"))

	return openapiclient.NewComposite(clients...)
}

// tryClusterOpenAPI attempts to get the OpenAPI client from the cluster's
// discovery endpoint. Returns nil if the call panics (e.g., fake discovery
// client in CLI test mode) or if the client is nil.
func (p *PolicyProcessor) tryClusterOpenAPI() (client openapi.Client) {
	defer func() {
		if r := recover(); r != nil {
			client = nil
		}
	}()
	return p.Client.GetKubeClient().Discovery().OpenAPIV3()
}

func (p *PolicyProcessor) resolveResource(kind string) (string, error) {
	gvk := schema.GroupVersionKind{Kind: kind}
	guessed, _ := meta.UnsafeGuessKindToResource(gvk)
	if guessed.Resource != "" {
		return guessed.Resource, nil
	}
	// Fallback: scan CEL policy match constraints for CRDs whose plural
	// cannot be derived from the kind name alone.
	kindLower := strings.ToLower(kind)
	for _, vp := range p.ValidatingPolicies {
		mc := vp.GetSpec().MatchConstraints
		if mc == nil {
			continue
		}
		for _, rr := range mc.ResourceRules {
			for _, res := range rr.Resources {
				base, _, _ := strings.Cut(res, "/")
				if strings.ToLower(base) == kindLower {
					return base, nil
				}
			}
		}
	}
	for _, mp := range p.MutatingPolicies {
		mc := mp.GetSpec().MatchConstraints
		if mc == nil {
			continue
		}
		for _, rr := range mc.ResourceRules {
			for _, res := range rr.Resources {
				base, _, _ := strings.Cut(res, "/")
				if strings.ToLower(base) == kindLower {
					return base, nil
				}
			}
		}
	}
	for _, gp := range p.GeneratingPolicies {
		mc := gp.GetSpec().MatchConstraints
		if mc == nil {
			continue
		}
		for _, rr := range mc.ResourceRules {
			for _, res := range rr.Resources {
				base, _, _ := strings.Cut(res, "/")
				if strings.ToLower(base) == kindLower {
					return base, nil
				}
			}
		}
	}
	return "", fmt.Errorf("failed to infer resource name for Kind %q: no RESTMapper available and no matching policy resource rule found", kind)
}

// targetMatchPredicate returns a predicate that checks whether a target resource
// matches a MutatingPolicy's targetMatchConstraints. This filters out policies
// whose target constraints don't match the given target, mirroring the filtering
// the background controller does via API list calls before calling Evaluate.
// namespace is the target's namespace, nil for a cluster-scoped target or an unknown namespace.
// celTargets maps policy keys (mpolengine.PolicyKey) to the set of target keys (celTargetKey) discovered
// by evaluating CEL expressions.
func targetMatchPredicate(m matching.Matcher, attr admission.Attributes, namespace *corev1.Namespace, celTargets map[string]map[string]bool) func(policiesv1beta1.MutatingPolicyLike) bool {
	return func(mpol policiesv1beta1.MutatingPolicyLike) bool {
		// like the background controller, a namespaced policy only ever acts on targets in its own
		// namespace, which also rules out cluster-scoped targets
		if ns := mpol.GetNamespace(); ns != "" && attr.GetNamespace() != ns {
			return false
		}
		tc := mpol.GetTargetMatchConstraints()
		if tc.Expression != "" {
			// CEL expression target selection — check pre-computed targets
			targets, ok := celTargets[mpolengine.PolicyKey(mpol)]
			if !ok {
				return false
			}
			// an object the expression returned without apiVersion and kind is still matched by namespace and name
			return targets[celTargetKey(attr.GetKind().GroupKind(), attr.GetNamespace(), attr.GetName())] ||
				targets[celTargetKey(schema.GroupKind{}, attr.GetNamespace(), attr.GetName())]
		}
		// Mirror background controller logic: use targetMatchConstraints if set,
		// otherwise fall back to matchConstraints
		constraints := mpol.GetMatchConstraints()
		if len(tc.ResourceRules) != 0 {
			constraints = tc.MatchResources
		}
		// Override operations to wildcard — operations are irrelevant for target matching
		// (the background controller doesn't check operations either)
		rules := make([]admissionregistrationv1.NamedRuleWithOperations, len(constraints.ResourceRules))
		for i, r := range constraints.ResourceRules {
			rules[i] = r
			rules[i].Operations = []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}
		}
		constraints.ResourceRules = rules
		matches, err := m.Match(&matching.MatchCriteria{Constraints: &constraints}, attr, namespace)
		if err != nil {
			return false
		}
		return matches
	}
}

// discoverCELTargets evaluates the CEL targetMatchConstraints expressions of the triggered mutateExisting
// policies and returns a map of policy key (mpolengine.PolicyKey) → set of target keys (celTargetKey).
func discoverCELTargets(
	provider mpolengine.Provider,
	contextProvider libs.Context,
	resource *unstructured.Unstructured,
	triggeredPolicies []string,
) (map[string]map[string]bool, error) {
	// only the policies this trigger triggered use the selections, so an expression written for another
	// trigger is never evaluated against this one (where it may well fail)
	triggered := sets.New(triggeredPolicies...)
	result := make(map[string]map[string]bool)
	// One shared cost budget for every target expression this call evaluates, so the CLI
	// accounts for them the way a controller does. Without it the programs below are built
	// through NewProgram but evaluated on a budgetless context, which charges nothing.
	ctx, releaseBudget := requestbudget.EnsureBackground(context.TODO())
	defer releaseBudget()
	pols := provider.Fetch(ctx, true)
	for _, pol := range pols {
		tc := pol.Policy.GetTargetMatchConstraints()
		if tc.Expression == "" || !triggered.Has(mpolengine.PolicyKey(pol.Policy)) {
			continue
		}

		compiledVars := pol.CompiledPolicy.GetCompiledVariables()
		data := map[string]any{
			compiler.ObjectKey: resource.Object,
		}
		vars := lazy.NewMapValue(compiler.VariablesType)
		data[compiler.VariablesKey] = vars
		for name, variable := range compiledVars {
			vars.Append(name, func(*lazy.MapValue) ref.Val {
				out, _, err := variable.ContextEval(ctx, data)
				if out != nil {
					return out
				}
				if err != nil {
					return types.WrapErr(err)
				}
				return nil
			})
		}

		policyNs := pol.Policy.GetNamespace()
		env, err := mpol.BuildMpolTargetEvalEnv(contextProvider, policyNs)
		if err != nil {
			return nil, fmt.Errorf("failed to build CEL env for policy %s: %w", pol.Policy.GetName(), err)
		}
		ast, issues := env.Compile(tc.Expression)
		if err := issues.Err(); err != nil {
			return nil, fmt.Errorf("failed to compile CEL expression for policy %s: %w", pol.Policy.GetName(), err)
		}
		if !ast.OutputType().IsExactType(types.NewMapType(types.StringType, types.AnyType)) {
			return nil, fmt.Errorf("output type of the target selector expression must be a map for policy %s", pol.Policy.GetName())
		}
		prog, err := compiler.NewProgram(env, ast)
		if err != nil {
			return nil, fmt.Errorf("failed to create CEL program for policy %s: %w", pol.Policy.GetName(), err)
		}
		out, _, err := prog.ContextEval(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate CEL expression for policy %s: %w", pol.Policy.GetName(), err)
		}
		unstructuredResources, err := celutils.ConvertToNative[map[string]interface{}](out)
		if err != nil {
			return nil, fmt.Errorf("failed to convert CEL result for policy %s: %w", pol.Policy.GetName(), err)
		}

		targetKeys := make(map[string]bool)
		if items, ok := unstructuredResources["items"].([]interface{}); ok {
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				obj := unstructured.Unstructured{Object: m}
				targetKeys[celTargetKey(obj.GroupVersionKind().GroupKind(), obj.GetNamespace(), obj.GetName())] = true
			}
		} else {
			obj := unstructured.Unstructured{Object: unstructuredResources}
			targetKeys[celTargetKey(obj.GroupVersionKind().GroupKind(), obj.GetNamespace(), obj.GetName())] = true
		}
		// keyed like policyKeyIn, so a clustered and a namespaced policy with the same name stay apart
		result[mpolengine.PolicyKey(pol.Policy)] = targetKeys
	}
	return result, nil
}

func hasSelector(match *admissionregistrationv1.MatchResources) bool {
	if match == nil {
		return false
	}
	if match.NamespaceSelector != nil {
		if len(match.NamespaceSelector.MatchLabels) > 0 || len(match.NamespaceSelector.MatchExpressions) > 0 {
			return false
		}
	}
	if match.ObjectSelector != nil {
		if len(match.ObjectSelector.MatchLabels) > 0 || len(match.ObjectSelector.MatchExpressions) > 0 {
			return false
		}
	}
	if len(match.ExcludeResourceRules) != 0 {
		return false
	}
	if len(match.ResourceRules) != 1 {
		return false
	}
	rule := match.ResourceRules[0]
	if len(rule.ResourceNames) > 0 {
		return false
	}
	return true
}

func (p *PolicyProcessor) getNamespaceResolver() celengine.NamespaceResolver {
	if p.Variables != nil {
		return p.Variables.Namespace
	}
	return func(string) *corev1.Namespace { return nil }
}

func (p *PolicyProcessor) loadCrds() error {
	for _, crdPath := range p.CrdPaths {
		if strings.TrimSpace(crdPath) == "" {
			continue
		}
		if err := common.LoadCrdsFromPath(crdPath); err != nil {
			return err
		}
	}
	return nil
}

func getAbsPath(path string) string {
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	absPath, _ := filepath.Abs(path)
	fileInfo, err := os.Stat(absPath)
	if err == nil && !fileInfo.IsDir() {
		absPath = filepath.Dir(absPath)
	}
	return absPath
}

// explain prints the decision trace of each policy that produced one. It prints nothing unless
// Explain is set, so it is safe to call unconditionally.
func (p *PolicyProcessor) explain(policies []celengine.ValidatingPolicyResponse) {
	if !p.Explain {
		return
	}
	for _, d := range selectTraces(policies) {
		// start on a fresh line: the mutate output printed before a trace ends without one
		fmt.Fprintln(p.Out)
		trace.Render(p.Out, d)
		fmt.Fprintln(p.Out)
	}
}

// selectTraces picks one trace per policy. A policy's autogenerated variants (Deployment,
// CronJob, ...) come back as separate responses under the same name, and printing all of them
// buries the answer under skipped copies. Per policy it keeps the first trace that applied to the
// resource, or the first one (the policy itself) when none did, in the order policies appeared.
func selectTraces(policies []celengine.ValidatingPolicyResponse) []*trace.Decision {
	var order []string
	chosen := map[string]*trace.Decision{}
	for _, r := range policies {
		if r.Trace == nil {
			continue
		}
		key := r.Policy.GetNamespace() + "/" + r.Policy.GetName()
		current, seen := chosen[key]
		if !seen {
			order = append(order, key)
			chosen[key] = r.Trace
			continue
		}
		if !current.Scope.Applied && r.Trace.Scope.Applied {
			chosen[key] = r.Trace
		}
	}
	selected := make([]*trace.Decision, 0, len(order))
	for _, key := range order {
		selected = append(selected, chosen[key])
	}
	return selected
}

// explainMutations is explain's counterpart for MutatingPolicy: same job, different response
// type, since mpolengine.MutatingPolicyResponse and celengine.ValidatingPolicyResponse aren't
// related types. Called once for the normal admission-triggered evaluation and once per
// mutate-existing target, so it prints incrementally as each is evaluated rather than only once
// mutation has finished across every policy and target.
func (p *PolicyProcessor) explainMutations(policies []mpolengine.MutatingPolicyResponse) {
	if !p.Explain {
		return
	}
	for _, d := range selectTracesMutations(policies) {
		// start on a fresh line: the mutate output printed before a trace ends without one
		fmt.Fprintln(p.Out)
		trace.Render(p.Out, d)
		fmt.Fprintln(p.Out)
	}
}

// selectTracesMutations is selectTraces' counterpart for MutatingPolicy -- see selectTraces for
// why one trace per policy, not one per autogenerated variant.
func selectTracesMutations(policies []mpolengine.MutatingPolicyResponse) []*trace.Decision {
	var order []string
	chosen := map[string]*trace.Decision{}
	for _, r := range policies {
		if r.Trace == nil {
			continue
		}
		key := r.Policy.GetNamespace() + "/" + r.Policy.GetName()
		current, seen := chosen[key]
		if !seen {
			order = append(order, key)
			chosen[key] = r.Trace
			continue
		}
		if !current.Scope.Applied && r.Trace.Scope.Applied {
			chosen[key] = r.Trace
		}
	}
	selected := make([]*trace.Decision, 0, len(order))
	for _, key := range order {
		selected = append(selected, chosen[key])
	}
	return selected
}
