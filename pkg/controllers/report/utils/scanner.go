package utils

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type scanner struct {
	logger logr.Logger
	engine engineapi.Engine
	config config.Configuration
	jp     jmespath.Interface
}

type ScanResult struct {
	EngineResponse *engineapi.EngineResponse
	Error          error
}

type Scanner interface {
	ScanResource(context.Context, unstructured.Unstructured, map[string]string, ...kyvernov1.PolicyInterface) map[kyvernov1.PolicyInterface]ScanResult
}

func NewScanner(
	logger logr.Logger,
	engine engineapi.Engine,
	config config.Configuration,
	jp jmespath.Interface,
) Scanner {
	return &scanner{
		logger: logger,
		engine: engine,
		config: config,
		jp:     jp,
	}
}

func (s *scanner) ScanResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policies ...kyvernov1.PolicyInterface) map[kyvernov1.PolicyInterface]ScanResult {
	results := map[kyvernov1.PolicyInterface]ScanResult{}
	for _, policy := range policies {
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
		results[policy] = ScanResult{response, multierr.Combine(errors...)}
	}
	return results
}

func (s *scanner) validateResource(ctx context.Context, resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*engineapi.EngineResponse, error) {
	enginectx := enginecontext.NewContext(s.jp)
	if err := enginectx.AddResource(resource.Object); err != nil {
		return nil, err
	}
	if err := enginectx.AddNamespace(resource.GetNamespace()); err != nil {
		return nil, err
	}
	if err := enginectx.AddImageInfos(&resource, s.config); err != nil {
		return nil, err
	}
	if err := enginectx.AddOperation("CREATE"); err != nil {
		return nil, err
	}
	policyCtx := engine.NewPolicyContextWithJsonContext(kyvernov1.Create, enginectx).
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
	enginectx := enginecontext.NewContext(s.jp)
	if err := enginectx.AddResource(resource.Object); err != nil {
		return nil, err
	}
	if err := enginectx.AddNamespace(resource.GetNamespace()); err != nil {
		return nil, err
	}
	if err := enginectx.AddImageInfos(&resource, s.config); err != nil {
		return nil, err
	}
	if err := enginectx.AddOperation("CREATE"); err != nil {
		return nil, err
	}
	policyCtx := engine.NewPolicyContextWithJsonContext(kyvernov1.Create, enginectx).
		WithNewResource(resource).
		WithPolicy(policy).
		WithNamespaceLabels(nsLabels)
	response, _ := s.engine.VerifyAndPatchImages(ctx, policyCtx)
	if len(response.PolicyResponse.Rules) > 0 {
		s.logger.Info("validateImages", "policy", policy, "response", response)
	}
	return &response, nil
}
