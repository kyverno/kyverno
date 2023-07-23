package utils

import (
	"context"

	"github.com/go-logr/logr"
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
	policies    kyvernov1.PolicyInterface
	vapPolicies v1alpha1.ValidatingAdmissionPolicy
	logger      logr.Logger
	engine      engineapi.Engine
	config      config.Configuration
	jp          jmespath.Interface
}

type ScanResult struct {
	EngineResponse *engineapi.EngineResponse
	Error           error
}

type Scanner interface {
	ScanResource(context.Context, unstructured.Unstructured, map[string]string) ScanResult
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

func (s *scanner) ScanResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string) ScanResult {
	results := ScanResult{}
	policy := s.policies
	vapPolicy := s.vapPolicies
		var errors []error
		logger := s.logger.WithValues("kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		response, err := s.validateResource(ctx, resource, nsLabels, policy)
		vapresponse, err := validatingadmissionpolicy.Validate(vapPolicy, resource)
		response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, vapresponse.PolicyResponse.Rules...)
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
		results = ScanResult{response, multierr.Combine(errors...)}
		}
	return results
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
		delete(annotations, "kyverno.io/verify-images")
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
