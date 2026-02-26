package processor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	json_patch "github.com/evanphx/json-patch/v5"
	"github.com/go-git/go-billy/v5"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/exceptions"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	celutils "github.com/kyverno/sdk/cel/utils"
	"gomodules.xyz/jsonpatch/v2"
	yamlv2 "gopkg.in/yaml.v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
	"k8s.io/client-go/openapi"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

type PolicyProcessor struct {
	Store                             *store.Store
	Policies                          []kyvernov1.PolicyInterface
	ValidatingAdmissionPolicies       []admissionregistrationv1.ValidatingAdmissionPolicy
	ValidatingAdmissionPolicyBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	MutatingAdmissionPolicies         []admissionregistrationv1beta1.MutatingAdmissionPolicy
	MutatingAdmissionPolicyBindings   []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	ValidatingPolicies                []policiesv1beta1.ValidatingPolicyLike
	GeneratingPolicies                []policiesv1beta1.GeneratingPolicyLike
	MutatingPolicies                  []policiesv1beta1.MutatingPolicyLike
	TargetResources                   []*unstructured.Unstructured
	Resource                          unstructured.Unstructured
	JsonPayload                       unstructured.Unstructured
	PolicyExceptions                  []*kyvernov2.PolicyException
	CELExceptions                     []*policiesv1beta1.PolicyException
	MutateLogPath                     string
	MutateLogPathIsDir                bool
	Variables                         *variables.Variables
	ParameterResources                []runtime.Object
	// TODO
	ContextFs                 billy.Filesystem
	ContextPath               string
	Cluster                   bool
	UserInfo                  *kyvernov2.RequestInfo
	PolicyReport              bool
	NamespaceSelectorMap      map[string]map[string]string
	Stdin                     bool
	Rc                        *ResultCounts
	PrintPatchResource        bool
	RuleToCloneSourceResource map[string]string
	Client                    dclient.Interface
	AuditWarn                 bool
	Subresources              []v1alpha1.Subresource
	Out                       io.Writer
	NamespaceCache            map[string]*unstructured.Unstructured
	ConfigMapResolver         engineapi.ConfigmapResolver
}

func (p *PolicyProcessor) ApplyPoliciesOnResource() ([]engineapi.EngineResponse, error) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	resource := p.Resource
	namespaceLabels := p.NamespaceSelectorMap[resource.GetNamespace()]
	if p.NamespaceCache == nil {
		p.NamespaceCache = make(map[string]*unstructured.Unstructured)
	}
	policyExceptionLister := &policyExceptionLister{
		exceptions: p.PolicyExceptions,
	}
	var client engineapi.Client
	if p.Client != nil {
		client = adapters.Client(p.Client)
	}
	rclient := p.Store.GetRegistryClient()
	if rclient == nil {
		rclient = registryclient.NewOrDie()
	}
	isCluster := false
	eng := engine.NewEngine(
		cfg,
		jmespath.New(cfg),
		client,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		store.ContextLoaderFactory(p.Store, p.ConfigMapResolver),
		exceptions.New(policyExceptionLister),
		&isCluster,
	)
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
	} else {
		if len(namespaceLabels) == 0 && resourceKind != "Namespace" && resourceNamespace != "" {
			var ns *unstructured.Unstructured
			var err error
			if cached, ok := p.NamespaceCache[resourceNamespace]; ok {
				ns = cached
			} else {
				ns, err = p.Client.GetResource(context.TODO(), "v1", "Namespace", "", resourceNamespace)
				if err != nil {
					log.Log.Error(err, "failed to get the resource's namespace")
					return nil, fmt.Errorf("failed to get the resource's namespace (%w)", err)
				}
				p.NamespaceCache[resourceNamespace] = ns
			}
			namespaceLabels = ns.GetLabels()
		}
	}
	resPath := fmt.Sprintf("%s/%s/%s", resourceNamespace, resourceKind, resourceName)
	responses := make([]engineapi.EngineResponse, 0, len(p.Policies))
	// mutate
	for _, policy := range p.Policies {
		if !policy.GetSpec().HasMutate() {
			continue
		}
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		mutateResponse := eng.Mutate(context.Background(), policyContext)
		err = p.processMutateEngineResponse(mutateResponse, resPath)
		if err != nil {
			return responses, fmt.Errorf("failed to print mutated result (%w)", err)
		}
		responses = append(responses, mutateResponse)
		resource = mutateResponse.PatchedResource
	}
	// verify images
	for _, policy := range p.Policies {
		if !policy.GetSpec().HasVerifyImages() {
			continue
		}
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		verifyImageResponse, verifiedImageData := eng.VerifyAndPatchImages(context.TODO(), policyContext)
		// update annotation to reflect verified images
		var patches []jsonpatch.JsonPatchOperation
		if !verifiedImageData.IsEmpty() {
			annotationPatches, err := verifiedImageData.Patches(len(verifyImageResponse.PatchedResource.GetAnnotations()) != 0, log.Log)
			if err != nil {
				return responses, err
			}
			// add annotation patches first
			patches = append(annotationPatches, patches...)
		}
		if len(patches) != 0 {
			patch := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)
			decoded, err := json_patch.DecodePatch(patch)
			if err != nil {
				return responses, err
			}
			options := &json_patch.ApplyOptions{SupportNegativeIndices: true, AllowMissingPathOnRemove: true, EnsurePathExistsOnAdd: true}
			resourceBytes, err := verifyImageResponse.PatchedResource.MarshalJSON()
			if err != nil {
				return responses, err
			}
			patchedResourceBytes, err := decoded.ApplyWithOptions(resourceBytes, options)
			if err != nil {
				return responses, err
			}
			if err := verifyImageResponse.PatchedResource.UnmarshalJSON(patchedResourceBytes); err != nil {
				return responses, err
			}
		}
		responses = append(responses, verifyImageResponse)
		resource = verifyImageResponse.PatchedResource
	}
	// validate
	for _, policy := range p.Policies {
		if !policyHasValidateOrVerifyImageChecks(policy) {
			continue
		}
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		validateResponse := eng.Validate(context.TODO(), policyContext)
		responses = append(responses, validateResponse)
		resource = validateResponse.PatchedResource
	}

	restMapper, err := utils.GetRESTMapper(p.Client)
	if err != nil {
		return nil, err
	}
	// Mutate Admission Policies
	if len(p.MutatingAdmissionPolicies) != 0 {
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
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
		compiler := mpolcompiler.NewCompiler()
		contextProvider, err := NewContextProvider(p.Client, restMapper, p.ContextFs, p.ContextPath, true, !p.Cluster)
		if err != nil {
			return nil, err
		}

		provider, err := mpolengine.NewProvider(compiler, p.MutatingPolicies, p.CELExceptions)
		if err != nil {
			return nil, err
		}
		if resource.Object != nil {
			tcm := mpolcompiler.NewStaticTypeConverterManager(p.openAPI())

			eng := mpolengine.NewEngine(provider, p.Variables.Namespace, matching.NewMatcher(), tcm, contextProvider)
			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
			}
			gvr := mapping.Resource
			var user authenticationv1.UserInfo
			if p.UserInfo != nil {
				user = p.UserInfo.AdmissionUserInfo
			}
			// create engine request
			request := celengine.Request(
				contextProvider,
				gvk,
				gvr,
				"",
				resource.GetName(),
				resource.GetNamespace(),
				admissionv1.Create,
				user,
				&resource,
				nil,
				false,
				nil,
			)
			reps, err := eng.Handle(context.TODO(), request, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to apply mutating policies on resource %s (%w)", resource.GetName(), err)
			}
			for _, r := range reps.Policies {
				if len(r.Rules) == 0 {
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
				p.Rc.addMutateResponse(response)

				err = p.processMutateEngineResponse(response, resPath)
				if err != nil {
					return responses, fmt.Errorf("failed to print mutated result (%w)", err)
				}

				responses = append(responses, response)
				resource = response.PatchedResource
			}
			// mutateExisting MutatingPolicies - process target resources
			if len(p.TargetResources) > 0 {
				// Create engine with nil matcher — targets are filtered by targetMatchConstraints
				// (via label selectors and CEL expressions) rather than by MatchConstraints which matches triggers
				mutExistEng := mpolengine.NewEngine(provider, p.Variables.Namespace, nil, tcm, contextProvider)
				targetMatcher := matching.NewMatcher()
				// Register target resources with FakeContextProvider so CEL resource.List()/resource.Get() can find them
				if fakeCtx, ok := contextProvider.(*libs.FakeContextProvider); ok {
					for _, target := range p.TargetResources {
						tGVK := target.GroupVersionKind()
						tMapping, err := restMapper.RESTMapping(tGVK.GroupKind(), tGVK.Version)
						if err != nil {
							continue
						}
						_ = fakeCtx.AddResource(tMapping.Resource, target)
					}
				}
				celTargets, err := discoverCELTargets(provider, contextProvider, &resource)
				if err != nil {
					return nil, fmt.Errorf("failed to discover CEL targets: %w", err)
				}
				for _, target := range p.TargetResources {
					targetGVK := target.GroupVersionKind()
					targetMapping, err := restMapper.RESTMapping(targetGVK.GroupKind(), targetGVK.Version)
					if err != nil {
						return nil, fmt.Errorf("failed to map target gvk to gvr %s (%v)\n", targetGVK, err)
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
					evalResponse, err := mutExistEng.Evaluate(context.TODO(), attr, request.Request, targetMatchPredicate(targetMatcher, attr, celTargets))
					if err != nil {
						return nil, fmt.Errorf("failed to evaluate mutateExisting policies on target %s (%w)", target.GetName(), err)
					}
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
							if rule.Status() == engineapi.RuleStatusPass {
								rules = append(rules, *rule.WithPatchedTarget(&patched, metav1.GroupVersionResource(targetGVR), ""))
							} else {
								rules = append(rules, rule)
							}
						}
						resp := engineapi.EngineResponse{
							Resource: resource,
							PolicyResponse: engineapi.PolicyResponse{
								Rules: rules,
							},
						}
						resp = resp.WithPolicy(engineapi.NewMutatingPolicyFromLike(r.Policy))
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
			return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
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
		compiler := vpolcompiler.NewCompiler()
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
		contextProvider, err := NewContextProvider(p.Client, restMapper, p.ContextFs, p.ContextPath, true, !p.Cluster)
		if err != nil {
			return nil, err
		}
		if resource.Object != nil {
			// Evaluate Kubernetes-mode policies via the admission path
			if len(k8sPolicies) > 0 {
				provider, err := vpolengine.NewProvider(compiler, k8sPolicies, p.CELExceptions)
				if err != nil {
					return nil, err
				}
				eng := vpolengine.NewEngine(provider, p.Variables.Namespace, matching.NewMatcher())
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
							return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
						}
						mapping.Resource.Resource = newR
					} else {
						return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
					}
				}

				gvr := mapping.Resource
				var user authenticationv1.UserInfo
				if p.UserInfo != nil {
					user = p.UserInfo.AdmissionUserInfo
				}
				// create engine request
				request := celengine.Request(
					contextProvider,
					gvk,
					gvr,
					// TODO: how to manage subresource ?
					"",
					resource.GetName(),
					resource.GetNamespace(),
					// TODO: how to manage other operations ?
					admissionv1.Create,
					user,
					&resource,
					nil,
					false,
					nil,
				)
				reps, err := eng.Handle(ctx, request, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to apply validating policies on resource %s (%w)", resource.GetName(), err)
				}
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
			// Also evaluate JSON-mode policies against the K8s resource as raw JSON
			if len(jsonPolicies) > 0 {
				provider, err := vpolengine.NewProvider(compiler, jsonPolicies, p.CELExceptions)
				if err != nil {
					return nil, err
				}
				eng := vpolengine.NewEngine(provider, nil, nil)
				request := celengine.RequestFromJSON(contextProvider, &resource)
				reps, err := eng.Handle(ctx, request, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to apply JSON-mode validating policies on resource %s (%w)", resource.GetName(), err)
				}
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
				provider, err := vpolengine.NewProvider(compiler, jsonPolicies, p.CELExceptions)
				if err != nil {
					return nil, err
				}
				eng := vpolengine.NewEngine(provider, nil, nil)
				request := celengine.RequestFromJSON(contextProvider, &unstructured.Unstructured{Object: p.JsonPayload.Object})
				reps, err := eng.Handle(ctx, request, nil)
				if err != nil {
					return nil, err
				}
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
		contextProvider, err := NewContextProvider(p.Client, restMapper, p.ContextFs, p.ContextPath, true, !p.Cluster)
		if err != nil {
			return nil, err
		}

		compiler := gpolcompiler.NewCompiler()
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
			engine := gpolengine.NewEngine(p.Variables.Namespace, matching.NewMatcher())
			// map gvk to gvr
			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
			}
			gvr := mapping.Resource
			var user authenticationv1.UserInfo
			if p.UserInfo != nil {
				user = p.UserInfo.AdmissionUserInfo
			}
			// create engine request
			request := celengine.Request(
				contextProvider,
				gvk,
				gvr,
				"",
				resource.GetName(),
				resource.GetNamespace(),
				admissionv1.Create,
				user,
				&resource,
				nil,
				false,
				nil,
			)
			for _, policy := range compiledPolicies {
				engineResponse, err := engine.Handle(request, policy, false)
				if err != nil {
					return nil, err
				}
				for _, res := range engineResponse.Policies {
					if res.Result == nil {
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
	// generate
	for _, policy := range p.Policies {
		if policy.GetSpec().HasGenerate() {
			policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
			if err != nil {
				return responses, err
			}
			generateResponse := eng.ApplyBackgroundChecks(context.TODO(), policyContext)
			if !generateResponse.IsEmpty() {
				newRuleResponse, err := handleGeneratePolicy(p.Out, p.Store, &generateResponse, *policyContext, p.RuleToCloneSourceResource)
				if err != nil {
					log.Log.Error(err, "failed to apply generate policy")
				} else {
					generateResponse.PolicyResponse.Rules = newRuleResponse
				}
				if err := p.processGenerateResponse(generateResponse, resPath); err != nil {
					return responses, err
				}
				responses = append(responses, generateResponse)
			}
			p.Rc.addGenerateResponse(generateResponse)
		}
	}
	p.Rc.addEngineResponses(p.AuditWarn, responses...)
	responses = append(responses, vapResponses...)
	return responses, nil
}

func (p *PolicyProcessor) makePolicyContext(
	jp jmespath.Interface,
	cfg config.Configuration,
	resource unstructured.Unstructured,
	policy kyvernov1.PolicyInterface,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
) (*policycontext.PolicyContext, error) {
	operation := kyvernov1.Create
	var resourceValues map[string]interface{}
	if p.Variables != nil {
		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(p.Out, policy, p.Variables.Subresources(), p.Client)
		vals, err := p.Variables.ComputeVariables(p.Store, policy.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied /*matches...*/)
		if err != nil {
			return nil, fmt.Errorf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag (%w)",
				policy.GetName(),
				resource.GetName(),
				err,
			)
		}
		resourceValues = vals
	}
	// TODO: this is kind of buggy, we should read that from the json context
	switch resourceValues["request.operation"] {
	case "DELETE":
		operation = kyvernov1.Delete
	case "UPDATE":
		operation = kyvernov1.Update
	}
	policyContext, err := engine.NewPolicyContext(
		jp,
		resource,
		operation,
		p.UserInfo,
		cfg,
	)
	if err != nil {
		log.Log.Error(err, "failed to create policy context")
		return nil, fmt.Errorf("failed to create policy context (%w)", err)
	}
	if operation == kyvernov1.Update {
		resource := resource.DeepCopy()
		policyContext = policyContext.WithOldResource(*resource)
		if err := policyContext.JSONContext().AddOldResource(resource.Object); err != nil {
			return nil, fmt.Errorf("failed to update old resource in json context (%w)", err)
		}
	}
	if operation == kyvernov1.Delete {
		policyContext = policyContext.WithOldResource(resource)
		if err := policyContext.JSONContext().AddOldResource(resource.Object); err != nil {
			return nil, fmt.Errorf("failed to update old resource in json context (%w)", err)
		}
	}
	policyContext = policyContext.
		WithPolicy(policy).
		WithNamespaceLabels(namespaceLabels).
		WithResourceKind(gvk, subresource)
	for key, value := range resourceValues {
		err = policyContext.JSONContext().AddVariable(key, value)
		if err != nil {
			log.Log.Error(err, "failed to add variable to context", "key", key, "value", value)
			return nil, fmt.Errorf("failed to add variable to context %s (%w)", key, err)
		}
	}
	// we need to get the resources back from the context to account for injected variables
	switch operation {
	case kyvernov1.Create:
		ret, err := policyContext.JSONContext().Query("request.object")
		if err != nil {
			return nil, err
		}
		if ret == nil {
			policyContext = policyContext.WithNewResource(unstructured.Unstructured{})
		} else {
			object, ok := ret.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("the object retrieved from the json context is not valid")
			}
			policyContext = policyContext.WithNewResource(unstructured.Unstructured{Object: object})
		}
	case kyvernov1.Update:
		{
			ret, err := policyContext.JSONContext().Query("request.object")
			if err != nil {
				return nil, err
			}
			if ret == nil {
				policyContext = policyContext.WithNewResource(unstructured.Unstructured{})
			} else {
				object, ok := ret.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("the object retrieved from the json context is not valid")
				}
				policyContext = policyContext.WithNewResource(unstructured.Unstructured{Object: object})
			}
		}
		{
			ret, err := policyContext.JSONContext().Query("request.oldObject")
			if err != nil {
				return nil, err
			}
			if ret == nil {
				policyContext = policyContext.WithOldResource(unstructured.Unstructured{})
			} else {
				object, ok := ret.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("the object retrieved from the json context is not valid")
				}
				policyContext = policyContext.WithOldResource(unstructured.Unstructured{Object: object})
			}
		}
	case kyvernov1.Delete:
		ret, err := policyContext.JSONContext().Query("request.oldObject")
		if err != nil {
			return nil, err
		}
		if ret == nil {
			policyContext = policyContext.WithOldResource(unstructured.Unstructured{})
		} else {
			object, ok := ret.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("the object retrieved from the json context is not valid")
			}
			policyContext = policyContext.WithOldResource(unstructured.Unstructured{Object: object})
		}
	}
	return policyContext, nil
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

func (p *PolicyProcessor) printOutput(resource interface{}, response engineapi.EngineResponse, resourcePath string, isGenerate bool) error {
	yamlEncodedResource, err := yamlv2.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal (%w)", err)
	}

	var yamlEncodedTargetResources [][]byte
	for _, ruleResponese := range response.PolicyResponse.Rules {
		patchedTarget, _, _ := ruleResponese.PatchedTarget()

		if patchedTarget != nil {
			yamlEncodedResource, err := yamlv2.Marshal(patchedTarget.Object)
			if err != nil {
				return fmt.Errorf("failed to marshal (%w)", err)
			}

			yamlEncodedResource = append(yamlEncodedResource, []byte("\n---\n")...)
			yamlEncodedTargetResources = append(yamlEncodedTargetResources, yamlEncodedResource)
		}
	}

	if p.MutateLogPath == "" {
		resource := string(yamlEncodedResource) + string("\n---")
		if len(strings.TrimSpace(resource)) > 0 {
			if !p.Stdin {
				fmt.Fprintf(p.Out, "\npolicy %s applied to %s:", response.Policy().GetName(), resourcePath)
			}
			fmt.Fprint(p.Out, "\n"+resource+"\n")
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
	if _, err := file.Write([]byte(string(yamlEncodedResource) + "\n---\n\n")); err != nil {
		return err
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
		return p.Client.GetKubeClient().Discovery().OpenAPIV3()
	}

	clients = append(clients, openapiclient.NewHardcodedBuiltins("1.32"))

	if crds, err := data.Crds(); err == nil {
		clients = append(clients, openapiclient.NewLocalSchemaFiles(crds))
	}

	return openapiclient.NewComposite(clients...)
}

func (p *PolicyProcessor) resolveResource(kind string) (string, error) {
	kindPrefix := strings.ToLower(kind)

	for _, newVp := range p.ValidatingPolicies {
		mc := newVp.GetSpec().MatchConstraints
		if mc == nil {
			continue
		}
		resRules := mc.ResourceRules
		for _, r := range resRules {
			for _, newR := range r.Resources {
				if strings.HasPrefix(strings.ToLower(newR), kindPrefix) {
					return newR, nil
				}
			}
		}
	}
	return "", fmt.Errorf("failed to get resource from %s", kind)
}

// targetMatchPredicate returns a predicate that checks whether a target resource
// matches a MutatingPolicy's targetMatchConstraints. This filters out policies
// whose target constraints don't match the given target, mirroring the filtering
// the background controller does via API list calls before calling Evaluate.
// celTargets maps policy names to the set of target keys (namespace/name) discovered
// by evaluating CEL expressions.
func targetMatchPredicate(m matching.Matcher, attr admission.Attributes, celTargets map[string]map[string]bool) func(policiesv1beta1.MutatingPolicyLike) bool {
	return func(mpol policiesv1beta1.MutatingPolicyLike) bool {
		tc := mpol.GetTargetMatchConstraints()
		if tc.Expression != "" {
			// CEL expression target selection — check pre-computed targets
			targets, ok := celTargets[mpol.GetName()]
			if !ok {
				return false
			}
			ns := attr.GetNamespace()
			name := attr.GetName()
			key := name
			if ns != "" {
				key = ns + "/" + name
			}
			return targets[key]
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
		matches, err := m.Match(&matching.MatchCriteria{Constraints: &constraints}, attr, nil)
		if err != nil {
			return false
		}
		return matches
	}
}

// discoverCELTargets evaluates CEL targetMatchConstraints expressions for mutateExisting
// policies and returns a map of policy name → set of target keys (namespace/name).
func discoverCELTargets(
	provider mpolengine.Provider,
	contextProvider libs.Context,
	resource *unstructured.Unstructured,
) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	pols := provider.Fetch(context.TODO(), true)
	for _, pol := range pols {
		tc := pol.Policy.GetTargetMatchConstraints()
		if tc.Expression == "" {
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
				out, _, err := variable.ContextEval(context.TODO(), data)
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
		prog, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("failed to create CEL program for policy %s: %w", pol.Policy.GetName(), err)
		}
		out, _, err := prog.ContextEval(context.TODO(), data)
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
				key := obj.GetName()
				if ns := obj.GetNamespace(); ns != "" {
					key = ns + "/" + key
				}
				targetKeys[key] = true
			}
		} else {
			obj := unstructured.Unstructured{Object: unstructuredResources}
			key := obj.GetName()
			if ns := obj.GetNamespace(); ns != "" {
				key = ns + "/" + key
			}
			targetKeys[key] = true
		}
		result[pol.Policy.GetName()] = targetKeys
	}
	return result, nil
}
