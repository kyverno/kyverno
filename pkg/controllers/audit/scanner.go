package audit

import (
	"errors"

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

func (s *scanner) mutate(resource unstructured.Unstructured, policy kyvernov1.PolicyInterface, jsonContext context.Interface) (*response.EngineResponse, error) {
	policyContext := &engine.PolicyContext{
		Policy:      policy,
		NewResource: resource,
		JSONContext: jsonContext,
		Client:      s.client,
		// TODO
		// ExcludeGroupRole: excludeGroupRole,
		// NamespaceLabels:  namespaceLabels,
	}
	engineResponse := engine.Mutate(policyContext)
	// TODO error handling looks strange
	if !engineResponse.IsSuccessful() {
		// log.V(4).Info("failed to apply mutation rules; reporting them")
		// return engineResponse, nil
		return nil, errors.New("failed to apply mutation rules")
	}
	// Verify if the JSON patches returned by the Mutate are already applied to the resource
	// if reflect.DeepEqual(resource, engineResponse.PatchedResource) {
	// 	// resources matches
	// 	// log.V(4).Info("resource already satisfies the policy")
	// 	// return engineResponse, nil
	// }
	return engineResponse, nil
	// return getFailedOverallRuleInfo(resource, engineResponse, log)
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
	// TODO what shall we do with responses ?
	_, err = s.mutate(resource, policy, ctx)
	if err != nil {
		// logger.Error(err, "failed to process mutation rule")
		return nil, err
	}
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
