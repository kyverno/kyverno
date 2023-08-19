package utils

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	"go.uber.org/multierr"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type scanner struct {
	logger      logr.Logger
	engine      engineapi.Engine
	config      config.Configuration
	jp          jmespath.Interface
	policies    kyvernov1.PolicyInterface
	vapPolicies v1alpha1.ValidatingAdmissionPolicy
}

type ScanResult struct {
	EngineResponse *engineapi.EngineResponse
	Error          error
}

type Scanner interface {
	ScanResource(context.Context, unstructured.Unstructured, map[string]string, kyvernov1.PolicyInterface) ScanResult
	ScanVAPResource(context.Context, unstructured.Unstructured, map[string]string, v1alpha1.ValidatingAdmissionPolicy) ScanResult
}

func NewScanner(
	logger logr.Logger,
	engine engineapi.Engine,
	config config.Configuration,
	jp jmespath.Interface,
	policies kyvernov1.PolicyInterface,
	vapPolicies v1alpha1.ValidatingAdmissionPolicy,
) Scanner {
	return &scanner{
		logger:      logger,
		engine:      engine,
		config:      config,
		jp:          jp,
		policies:    policies,
		vapPolicies: vapPolicies,
	}
}

func (s *scanner) ScanResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) ScanResult {
	var errors []error
	logger := s.logger.WithValues("kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	response, err := s.validateResource(ctx, resource, nsLabels, policy)
	if err != nil {
		logger.Error(err, "failed to scan resource")
		errors = append(errors, err)
	}
	spec := policy.GetSpec()
	if spec.HasVerifyImages() {
		ivResponse, err := s.validateImages(ctx, resource, nsLabels, policy)
		if err != nil {
			logger.Error(err, "failed to scan images")
			errors = append(errors, err)
		}
		if response == nil {
			response = ivResponse
		} else if ivResponse != nil {
			response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ivResponse.PolicyResponse.Rules...)
		}
	}
return ScanResult{response, multierr.Combine(errors...)}
}

func (s *scanner) ScanVAPResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, vapPolicy v1alpha1.ValidatingAdmissionPolicy) ScanResult {
	var errors []error
	logger := s.logger.WithValues("kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	vapresponse, err := validatingadmissionpolicy.Validate(vapPolicy, resource)
	if err != nil {
		logger.Error(err, "failed to validate ValidatingAdmissionPolicy")
		errors = append(errors, err)
	}
	return ScanResult{vapresponse, multierr.Combine(errors...)}
}

func (s *scanner) validateResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*engineapi.EngineResponse, error) {
	policyCtx, err := engine.NewPolicyContext(s.jp, resource, kyvernov1.Create, nil, s.config)
	if err != nil {
		return nil, err
	}
	policyCtx = policyCtx.
		WithNewResource(resource).
		WithPolicy(policy).
		WithNamespaceLabels(nsLabels)
	response := s.engine.Validate(ctx, policyCtx)
	return &response, nil
}

func (s *scanner) validateImages(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*engineapi.EngineResponse, error) {
	annotations := resource.GetAnnotations()
	if annotations != nil {
		resource = *resource.DeepCopy()
		delete(annotations, kyverno.AnnotationImageVerify)
		resource.SetAnnotations(annotations)
	}
	policyCtx, err := engine.NewPolicyContext(s.jp, resource, kyvernov1.Create, nil, s.config)
	if err != nil {
		return nil, err
	}
	policyCtx = policyCtx.
		WithNewResource(resource).
		WithPolicy(policy).
		WithNamespaceLabels(nsLabels)
	response, _ := s.engine.VerifyAndPatchImages(ctx, policyCtx)
	if len(response.PolicyResponse.Rules) > 0 {
		s.logger.Info("validateImages", "policy", policy, "response", response)
	}
	return &response, nil
}
