package audit

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
	logger logr.Logger
	client dclient.Interface
}

type ScanResult struct {
	*response.EngineResponse
	error
}

type Scanner interface {
	Scan(unstructured.Unstructured, ...kyvernov1.PolicyInterface) map[kyvernov1.PolicyInterface]ScanResult
}

func NewScanner(logger logr.Logger, client dclient.Interface) Scanner {
	return &scanner{
		logger: logger,
		client: client,
	}
}

func (s *scanner) Scan(resource unstructured.Unstructured, policies ...kyvernov1.PolicyInterface) map[kyvernov1.PolicyInterface]ScanResult {
	results := map[kyvernov1.PolicyInterface]ScanResult{}
	for _, policy := range policies {
		response, err := s.scan(resource, policy)
		if err != nil {
			s.logger.Error(err, "failed to scan resource")
		}
		results[policy] = ScanResult{response, err}
	}
	return results
}

func (s *scanner) scan(resource unstructured.Unstructured, policy kyvernov1.PolicyInterface) (*response.EngineResponse, error) {
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
	// TODO: mutation
	// engineResponseMutation, err = mutation(policy, resource, logger, ctx, namespaceLabels)
	// if err != nil {
	// 	logger.Error(err, "failed to process mutation rule")
	// }

	policyCtx := &engine.PolicyContext{
		Policy:      policy,
		NewResource: resource,
		JSONContext: ctx,
		Client:      s.client,
		// TODO
		// ExcludeGroupRole: excludeGroupRole,
		// NamespaceLabels:  namespaceLabels,
	}
	return engine.Validate(policyCtx), nil
}
