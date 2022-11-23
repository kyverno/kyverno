package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type scanner struct {
	logger           logr.Logger
	client           dclient.Interface
	excludeGroupRole []string
}

type ScanResult struct {
	EngineResponse *response.EngineResponse
	Error          error
}

type Scanner interface {
	ScanResource(unstructured.Unstructured, map[string]string, ...kyvernov1.PolicyInterface) map[kyvernov1.PolicyInterface]ScanResult
}

func NewScanner(logger logr.Logger, client dclient.Interface, excludeGroupRole ...string) Scanner {
	return &scanner{
		logger:           logger,
		client:           client,
		excludeGroupRole: excludeGroupRole,
	}
}

func (s *scanner) ScanResource(resource unstructured.Unstructured, nsLabels map[string]string, policies ...kyvernov1.PolicyInterface) map[kyvernov1.PolicyInterface]ScanResult {
	results := map[kyvernov1.PolicyInterface]ScanResult{}
	for _, policy := range policies {
		var errors []error
		response, err := s.validateResource(resource, nsLabels, policy)
		if err != nil {
			s.logger.Error(err, "failed to scan resource")
			errors = append(errors, err)
		}
		spec := policy.GetSpec()
		if spec.HasVerifyImages() {
			ivResponse, err := s.validateImages(resource, nsLabels, policy)
			if err != nil {
				s.logger.Error(err, "failed to scan images")
				errors = append(errors, err)
			}
			if response == nil {
				response = ivResponse
			} else {
				response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ivResponse.PolicyResponse.Rules...)
			}
		}
		results[policy] = ScanResult{response, multierr.Combine(errors...)}
	}
	return results
}

func (s *scanner) validateResource(resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*response.EngineResponse, error) {
	ctx := context.NewContext()
	if err := ctx.AddResource(resource.Object); err != nil {
		return nil, err
	}
	if err := ctx.AddNamespace(resource.GetNamespace()); err != nil {
		return nil, err
	}
	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, err
	}
	if err := ctx.AddOperation("CREATE"); err != nil {
		return nil, err
	}
	policyCtx := &engine.PolicyContext{
		Policy:           policy,
		NewResource:      resource,
		JSONContext:      ctx,
		Client:           s.client,
		NamespaceLabels:  nsLabels,
		ExcludeGroupRole: s.excludeGroupRole,
	}
	return engine.Validate(policyCtx), nil
}

func (s *scanner) validateImages(resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*response.EngineResponse, error) {
	ctx := context.NewContext()
	if err := ctx.AddResource(resource.Object); err != nil {
		return nil, err
	}
	if err := ctx.AddNamespace(resource.GetNamespace()); err != nil {
		return nil, err
	}
	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, err
	}
	if err := ctx.AddOperation("CREATE"); err != nil {
		return nil, err
	}
	policyCtx := &engine.PolicyContext{
		Policy:           policy,
		NewResource:      resource,
		JSONContext:      ctx,
		Client:           s.client,
		NamespaceLabels:  nsLabels,
		ExcludeGroupRole: s.excludeGroupRole,
	}
	response, _ := engine.VerifyAndPatchImages(policyCtx)
	if len(response.PolicyResponse.Rules) > 0 {
		s.logger.Info("validateImages", "policy", policy, "response", response)
	}
	return response, nil
}
