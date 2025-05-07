package mutation

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type mutateResourceHandler struct{}

func NewMutateResourceHandler() (handlers.Handler, error) {
	return mutateResourceHandler{}, nil
}

func (h mutateResourceHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(exceptions, policyContext, logger)
	if len(matchedExceptions) > 0 {
		var keys []string
		for i, exception := range matchedExceptions {
			key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
				return resource, handlers.WithError(rule, engineapi.Mutation, "failed to compute exception key", err)
			}
			keys = append(keys, key)
		}

		logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
		return resource, handlers.WithResponses(
			engineapi.RuleSkip(rule.Name, engineapi.Mutation, "rule is skipped due to policy exceptions"+strings.Join(keys, ", "), rule.ReportProperties).WithExceptions(matchedExceptions),
		)
	}

	_, subresource := policyContext.ResourceKind()
	logger.V(3).Info("processing mutate rule")
	var parentResourceGVR metav1.GroupVersionResource
	if subresource != "" {
		parentResourceGVR = policyContext.RequestResource()
	}
	resourceInfo := resourceInfo{
		unstructured:      resource,
		subresource:       subresource,
		parentResourceGVR: parentResourceGVR,
	}
	// logger.V(4).Info("apply rule to resource", "resource namespace", patchedResource.unstructured.GetNamespace(), "resource name", patchedResource.unstructured.GetName())
	var mutateResp *mutate.Response
	if rule.Mutation.ForEachMutation != nil {
		m := &forEachMutator{
			rule:          rule,
			foreach:       rule.Mutation.ForEachMutation,
			policyContext: policyContext,
			resource:      resourceInfo,
			logger:        logger,
			contextLoader: contextLoader,
			nesting:       0,
		}
		mutateResp = m.mutateForEach(ctx)
	} else {
		mutateResp = mutate.Mutate(&rule, policyContext.JSONContext(), resource, logger)
	}
	if mutateResp == nil {
		return resource, nil
	}
	return mutateResp.PatchedResource, handlers.WithResponses(buildRuleResponse(&rule, mutateResp, resourceInfo))
}
