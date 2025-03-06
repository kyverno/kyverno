package engine

import (
	"context"

	"golang.org/x/exp/maps"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ImageVerifyEngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []ImageVerifyPolicyResponse
}

type ImageVerifyPolicyResponse struct {
	Policy *policiesv1alpha1.ImageVerificationPolicy
	Result engineapi.RuleResponse
}

type ImageVerifyEngine interface {
	HandleMutating(context.Context, EngineRequest) (ImageVerifyEngineResponse, error)
}

type ivengine struct {
	logger       logr.Logger
	provider     ImageVerifyPolProviderFunc
	nsResolver   NamespaceResolver
	matcher      matching.Matcher
	lister       k8scorev1.SecretInterface
	registryOpts []imagedataloader.Option
}

func NewImageVerifyEngine(logger logr.Logger, provider ImageVerifyPolProviderFunc, nsResolver NamespaceResolver, matcher matching.Matcher, lister k8scorev1.SecretInterface, registryOpts []imagedataloader.Option) ImageVerifyEngine {
	return &ivengine{
		logger:       logger,
		provider:     provider,
		nsResolver:   nsResolver,
		matcher:      matcher,
		lister:       lister,
		registryOpts: registryOpts,
	}
}

func (e *ivengine) HandleMutating(ctx context.Context, request EngineRequest) (ImageVerifyEngineResponse, error) {
	var response ImageVerifyEngineResponse
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
	responses, err := e.handlePolicy(ctx, policies, attr, &request.request, namespace, request.context)
	if err != nil {
		return response, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, nil
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

func (e *ivengine) handlePolicy(ctx context.Context, policies []CompiledImageVerificationPolicy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context contextlib.ContextInterface) ([]ImageVerifyPolicyResponse, error) {
	responses := make(map[string]ImageVerifyPolicyResponse)
	if e.matcher != nil {
		for _, pol := range policies {
			matches, err := e.matchPolicy(pol, attr, namespace)
			response := ImageVerifyPolicyResponse{
				Policy: pol.Policy,
			}
			if err != nil {
				response.Result = *engineapi.RuleError("match", engineapi.ImageVerify, "failed to execute matching", err, nil)
				responses[pol.Policy.GetName()] = response
			} else if !matches {
				responses[pol.Policy.GetName()] = response
			}
		}
	}

	ictx, err := imagedataloader.NewImageContext(e.lister, e.registryOpts...)
	if err != nil {
		return nil, err
	}

	c := eval.NewCompiler(ictx, e.lister, request.RequestResource)
	results := make(map[string]ImageVerifyPolicyResponse, len(policies))
	for _, ivpol := range policies {
		response := ImageVerifyPolicyResponse{
			Policy: ivpol.Policy,
		}

		if p, errList := c.Compile(e.logger, ivpol.Policy); errList != nil {
			response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to compile policy", err, nil)
		} else {
			result, err := p.Evaluate(ctx, ictx, attr, request, namespace, true)
			if err != nil {
				response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to evaluate policy", err, nil)
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
		results[ivpol.Policy.GetName()] = response
	}

	return maps.Values(responses), nil
}
