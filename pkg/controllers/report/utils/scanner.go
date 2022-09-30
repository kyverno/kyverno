package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
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
		response, err := s.scan(resource, nsLabels, policy)
		if err != nil {
			s.logger.Error(err, "failed to scan resource")
		}
		results[policy] = ScanResult{response, err}
	}
	return results
}

func (s *scanner) scan(resource unstructured.Unstructured, nsLabels map[string]string, policy kyvernov1.PolicyInterface) (*response.EngineResponse, error) {
	ctx := context.NewContext()
	err := ctx.AddResource(resource.Object)
	if err != nil {
		return nil, err
	}
	err = ctx.AddNamespace(resource.GetNamespace())
	if err != nil {
		return nil, err
	}
	if err := ctx.AddImageInfos(&resource); err != nil {
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
