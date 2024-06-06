package engine

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	matched "github.com/kyverno/kyverno/pkg/utils/match"
	"k8s.io/client-go/tools/cache"
)

func findExceptions(
	selector engineapi.PolicyExceptionSelector,
	policy kyvernov1.PolicyInterface,
	rule string,
) ([]*kyvernov2alpha1.PolicyException, error) {
	if selector == nil {
		return nil, nil
	}
	return selector.Find(cache.MetaObjectToName(policy).String(), rule)
}

// matchesException checks if an exception applies to the resource being admitted
func matchesException(
	selector engineapi.PolicyExceptionSelector,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
) (*kyvernov2alpha1.PolicyException, error) {
	candidates, err := findExceptions(selector, policyContext.Policy(), rule.Name)
	if err != nil {
		return nil, err
	}
	gvk, subresource := policyContext.ResourceKind()
	resource := policyContext.NewResource()
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	for _, candidate := range candidates {
		err := matched.CheckMatchesResources(
			resource,
			candidate.Spec.Match,
			policyContext.NamespaceLabels(),
			policyContext.AdmissionInfo(),
			gvk,
			subresource,
		)
		// if there's no error it means a match
		if err == nil {
			return candidate, nil
		}
	}
	return nil, nil
}

// hasPolicyExceptions returns nil when there are no matching exceptions.
// A rule response is returned when an exception is matched, or there is an error.
func (e *engine) hasPolicyExceptions(
	logger logr.Logger,
	ruleType engineapi.RuleType,
	ctx engineapi.PolicyContext,
	rule kyvernov1.Rule,
) *engineapi.RuleResponse {
	// if matches, check if there is a corresponding policy exception
	exception, err := matchesException(e.exceptionSelector, ctx, rule)
	if err != nil {
		logger.Error(err, "failed to match exceptions")
		return nil
	}
	if exception == nil {
		return nil
	}
	key, err := cache.MetaNamespaceKeyFunc(exception)
	if err != nil {
		logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
		return engineapi.RuleError(rule.Name, ruleType, "failed to compute exception key", err)
	} else {
		logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
		return engineapi.RuleSkip(rule.Name, ruleType, "rule skipped due to policy exception "+key).WithException(exception)
	}
}
