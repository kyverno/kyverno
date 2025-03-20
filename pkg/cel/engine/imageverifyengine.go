package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	resourcelib "github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"golang.org/x/exp/maps"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
)

type ImageVerifyEngine interface {
	HandleMutating(context.Context, EngineRequest) (eval.ImageVerifyEngineResponse, []jsonpatch.JsonPatchOperation, error)
	HandleValidating(ctx context.Context, request EngineRequest) (eval.ImageVerifyEngineResponse, error)
}

type ivengine struct {
	provider     ImageVerifyPolProviderFunc
	nsResolver   NamespaceResolver
	matcher      matching.Matcher
	lister       k8scorev1.SecretInterface
	registryOpts []imagedataloader.Option
}

func NewImageVerifyEngine(provider ImageVerifyPolProviderFunc, nsResolver NamespaceResolver, matcher matching.Matcher, lister k8scorev1.SecretInterface, registryOpts []imagedataloader.Option) ImageVerifyEngine {
	return &ivengine{
		provider:     provider,
		nsResolver:   nsResolver,
		matcher:      matcher,
		lister:       lister,
		registryOpts: registryOpts,
	}
}

func (e *ivengine) HandleValidating(ctx context.Context, request EngineRequest) (eval.ImageVerifyEngineResponse, error) {
	var response eval.ImageVerifyEngineResponse
	// fetch compiled policies
	policies, err := e.provider.ImageVerificationPolicies(ctx)
	if err != nil {
		return response, err
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
	}
	// default dry run
	dryRun := false
	if request.request.DryRun != nil {
		dryRun = *request.request.DryRun
	}
	// create admission attributes
	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.request.Kind),
		request.request.Namespace,
		request.request.Name,
		schema.GroupVersionResource(request.request.Resource),
		request.request.SubResource,
		admission.Operation(request.request.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	responses, err := e.handleValidation(policies, attr, namespace)
	if err != nil {
		return response, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, nil
}

func (e *ivengine) HandleMutating(ctx context.Context, request EngineRequest) (eval.ImageVerifyEngineResponse, []jsonpatch.JsonPatchOperation, error) {
	var response eval.ImageVerifyEngineResponse
	// fetch compiled policies
	policies, err := e.provider.ImageVerificationPolicies(ctx)
	if err != nil {
		return response, nil, err
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.request)
	if err != nil {
		return response, nil, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
	}
	// default dry run
	dryRun := false
	if request.request.DryRun != nil {
		dryRun = *request.request.DryRun
	}
	// create admission attributes
	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.request.Kind),
		request.request.Namespace,
		request.request.Name,
		schema.GroupVersionResource(request.request.Resource),
		request.request.SubResource,
		admission.Operation(request.request.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	responses, patches, err := e.handleMutation(ctx, policies, attr, &request.request, namespace, request.context)
	if err != nil {
		return response, nil, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, patches, nil
}

func (e *ivengine) matchPolicy(policy CompiledImageVerificationPolicy, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	match := func(constraints *admissionregistrationv1.MatchResources) (bool, error) {
		criteria := matchCriteria{constraints: constraints}
		matches, err := e.matcher.Match(&criteria, attr, namespace)
		if err != nil {
			return false, err
		}
		return matches, nil
	}

	// match against main policy constraints
	matches, err := match(policy.Policy.Spec.MatchConstraints)
	if err != nil {
		return false, err
	}
	if matches {
		return true, nil
	}

	return false, nil
}

func (e *ivengine) handleMutation(ctx context.Context, policies []CompiledImageVerificationPolicy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context resourcelib.ContextInterface) ([]eval.ImageVerifyPolicyResponse, []jsonpatch.JsonPatchOperation, error) {
	results := make(map[string]eval.ImageVerifyPolicyResponse, len(policies))
	filteredPolicies := make([]CompiledImageVerificationPolicy, 0)
	if e.matcher != nil {
		for _, pol := range policies {
			matches, err := e.matchPolicy(pol, attr, namespace)
			response := eval.ImageVerifyPolicyResponse{
				Policy:     pol.Policy,
				Actions:    pol.Actions,
				Exceptions: pol.Exceptions,
			}
			if err != nil {
				response.Result = *engineapi.RuleError("match", engineapi.ImageVerify, "failed to execute matching", err, nil)
				results[pol.Policy.GetName()] = response
			} else if matches {
				filteredPolicies = append(filteredPolicies, pol)
			}
		}
	}

	ictx, err := imagedataloader.NewImageContext(e.lister, e.registryOpts...)
	if err != nil {
		return nil, nil, err
	}

	c := eval.NewCompiler(ictx, e.lister, request.RequestResource)
	for _, ivpol := range filteredPolicies {
		response := eval.ImageVerifyPolicyResponse{
			Policy:     ivpol.Policy,
			Actions:    ivpol.Actions,
			Exceptions: ivpol.Exceptions,
		}

		if p, errList := c.Compile(ivpol.Policy, ivpol.Exceptions); errList != nil {
			response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to compile policy", errList.ToAggregate(), nil)
		} else {
			result, err := p.Evaluate(ctx, ictx, attr, request, namespace, true)
			if err != nil {
				response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to evaluate policy", err, nil)
			} else if result != nil {
				if len(result.Exceptions) > 0 {
					exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
					var keys []string
					for i := range result.Exceptions {
						key, err := cache.MetaNamespaceKeyFunc(&result.Exceptions[i])
						if err != nil {
							response.Result = *engineapi.RuleError("exception", engineapi.Validation, "failed to compute exception key", err, nil)
						}
						keys = append(keys, key)
						exceptions = append(exceptions, engineapi.NewCELPolicyException(result.Exceptions[i]))
					}
					response.Result = *engineapi.RuleSkip("exception", engineapi.Validation, "rule is skipped due to policy exception: "+strings.Join(keys, ", "), nil).WithExceptions(exceptions)
				} else {
					ruleName := ivpol.Policy.GetName()
					if result.Error != nil {
						response.Result = *engineapi.RuleError(ruleName, engineapi.ImageVerify, "error", err, nil)
					} else if result.Result {
						response.Result = *engineapi.RulePass(ruleName, engineapi.ImageVerify, "success", nil)
					} else {
						response.Result = *engineapi.RuleFail(ruleName, engineapi.ImageVerify, result.Message, nil)
					}
				}
			}
		}
		results[ivpol.Policy.GetName()] = response
	}

	ann, err := objectAnnotations(attr)
	if err != nil {
		return nil, nil, err
	}
	patches, err := eval.MakeImageVerifyOutcomePatch(len(ann) != 0, results)
	if err != nil {
		return nil, nil, err
	}

	return maps.Values(results), patches, nil
}

func objectAnnotations(attr admission.Attributes) (map[string]string, error) {
	obj := attr.GetObject()
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return nil, nil
	}

	ret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{Object: ret}
	return u.GetAnnotations(), nil
}

func (e *ivengine) handleValidation(policies []CompiledImageVerificationPolicy, attr admission.Attributes, namespace runtime.Object) ([]eval.ImageVerifyPolicyResponse, error) {
	responses := make(map[string]eval.ImageVerifyPolicyResponse)
	annotations, err := objectAnnotations(attr)
	if err != nil {
		return nil, err
	}
	if len(annotations) == 0 {
		return nil, fmt.Errorf("annotations not present on object, image verification failed")
	}

	filteredPolicies := make([]CompiledImageVerificationPolicy, 0)
	if e.matcher != nil {
		for _, pol := range policies {
			matches, err := e.matchPolicy(pol, attr, namespace)
			response := eval.ImageVerifyPolicyResponse{
				Policy:     pol.Policy,
				Actions:    pol.Actions,
				Exceptions: pol.Exceptions,
			}
			if err != nil {
				response.Result = *engineapi.RuleError("match", engineapi.ImageVerify, "failed to execute matching", err, nil)
				responses[pol.Policy.GetName()] = response
			} else if matches {
				filteredPolicies = append(filteredPolicies, pol)
			}
		}
	}

	if data, found := annotations[kyverno.AnnotationImageVerifyOutcomes]; !found {
		return nil, fmt.Errorf("%s annotation not present", kyverno.AnnotationImageVerifyOutcomes)
	} else {
		var outcomes map[string]eval.ImageVerificationOutcome
		if err := json.Unmarshal([]byte(data), &outcomes); err != nil {
			return nil, err
		}

		for _, pol := range filteredPolicies {
			resp := eval.ImageVerifyPolicyResponse{
				Policy:  pol.Policy,
				Actions: pol.Actions,
			}
			if o, found := outcomes[pol.Policy.GetName()]; !found {
				resp.Result = *engineapi.RuleFail(pol.Policy.GetName(), engineapi.ImageVerify, "policy not evaluated", nil)
			} else {
				resp.Result = *engineapi.NewRuleResponse(o.Name, engineapi.ImageVerify, o.Message, o.Status, o.Properties)
			}
			responses[pol.Policy.GetName()] = resp
		}
	}
	return maps.Values(responses), nil
}
