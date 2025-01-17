package engine

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EngineRequest struct {
	Resource        *unstructured.Unstructured
	NamespaceLabels map[string]map[string]string
}

type EngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []PolicyResponse
}

type PolicyResponse struct {
	Policy kyvernov2alpha1.ValidatingPolicy
	Rules  []engineapi.RuleResponse
}

type ValidatingPolicy struct {
	policy kyvernov2alpha1.ValidatingPolicy
}

func (p *ValidatingPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return nil
}

func (p *ValidatingPolicy) AsValidatingAdmissionPolicy() *admissionregistrationv1beta1.ValidatingAdmissionPolicy {
	return nil
}

func (p *ValidatingPolicy) GetType() engineapi.PolicyType {
	return engineapi.ValidatingAdmissionPolicyType
}

func (p *ValidatingPolicy) GetAPIVersion() string {
	return "admissionregistration.k8s.io/v1beta1"
}

func (p *ValidatingPolicy) GetName() string {
	return p.policy.GetName()
}

func (p *ValidatingPolicy) GetNamespace() string {
	return p.policy.GetNamespace()
}

func (p *ValidatingPolicy) GetKind() string {
	return "ValidatingAdmissionPolicy"
}

func (p *ValidatingPolicy) GetResourceVersion() string {
	return p.policy.GetResourceVersion()
}

func (p *ValidatingPolicy) GetAnnotations() map[string]string {
	return p.policy.GetAnnotations()
}

func (p *ValidatingPolicy) IsNamespaced() bool {
	return false
}

func (p *ValidatingPolicy) MetaObject() metav1.Object {
	return &p.policy
}

func NewValidatingPolicy(pol kyvernov2alpha1.ValidatingPolicy) engineapi.GenericPolicy {
	return &ValidatingPolicy{
		policy: pol,
	}
}

type Engine interface {
	Handle(context.Context, EngineRequest, ...policy.CompiledPolicy) (EngineResponse, error)
}

type engine struct {
	provider Provider
}

func NewEngine(provider Provider) *engine {
	return &engine{
		provider: provider,
	}
}

func (e *engine) Handle(ctx context.Context, request EngineRequest) (EngineResponse, error) {
	response := EngineResponse{
		Resource: request.Resource,
	}
	policies, err := e.provider.CompiledPolicies(ctx)
	if err != nil {
		return response, err
	}
	for _, policy := range policies {
		response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, request))
	}
	return response, nil
}

func (e *engine) handlePolicy(ctx context.Context, policy policy.CompiledPolicy, request EngineRequest) PolicyResponse {
	var rules []engineapi.RuleResponse
	ok, err := policy.Evaluate(ctx, request.Resource, request.NamespaceLabels)
	// TODO: evaluation should be per rule
	if err != nil {
		rules = handlers.WithResponses(engineapi.RuleError("todo", engineapi.Validation, "failed to load context", err, nil))
	} else if ok {
		rules = handlers.WithResponses(engineapi.RulePass("todo", engineapi.Validation, "success", nil))
	} else {
		rules = handlers.WithResponses(engineapi.RuleFail("todo", engineapi.Validation, "failure", nil))
	}
	return PolicyResponse{
		// TODO
		Policy: kyvernov2alpha1.ValidatingPolicy{},
		Rules:  rules,
	}
}
