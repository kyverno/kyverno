package engine

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/pss"
	matched "github.com/kyverno/kyverno/pkg/utils/match"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// GetPolicyExceptions get all exceptions that match both the policy and the rule.
// It returns two groups of exceptions:
// 1. The 1st for exceptions that need to be executed before applying the actual policy.
// 2. The 2nd for exceptions that need to be executed after applying the actual policy like podSecurity.
func (e *engine) GetPolicyExceptions(
	policy kyvernov1.PolicyInterface,
	rule string,
) ([]kyvernov2beta1.PolicyException, []kyvernov2beta1.PolicyException, error) {
	var preprocessExceptions, postprocessExceptions []kyvernov2beta1.PolicyException
	if e.exceptionSelector == nil {
		return preprocessExceptions, postprocessExceptions, nil
	}
	polexs, err := e.exceptionSelector.List(labels.Everything())
	if err != nil {
		return preprocessExceptions, postprocessExceptions, err
	}
	policyName, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		return preprocessExceptions, postprocessExceptions, fmt.Errorf("failed to compute policy key: %w", err)
	}
	for _, polex := range polexs {
		if polex.Contains(policyName, rule) {
			if polex.HasPodSecurity() {
				postprocessExceptions = append(postprocessExceptions, *polex)
			} else {
				preprocessExceptions = append(preprocessExceptions, *polex)
			}
		}
	}
	return preprocessExceptions, postprocessExceptions, nil
}

// PreprocessPolicyExceptions is used before the execution of a policy on a resource
func PreprocessPolicyExceptions(
	logger logr.Logger,
	ruleType engineapi.RuleType,
	polexs []kyvernov2beta1.PolicyException,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
) *engineapi.RuleResponse {
	exception := matchesException(polexs, policyContext)
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

// PostprocessPolicyExceptions is used after the execution of a policy on a resource in case of validate.podSecurity rule.
func PostprocessPolicyExceptions(
	logger logr.Logger,
	ruleType engineapi.RuleType,
	polexs []kyvernov2beta1.PolicyException,
	podSecurityChecks *engineapi.PodSecurityChecks,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
) *engineapi.RuleResponse {
	exception := matchesException(polexs, policyContext)
	if exception == nil {
		return nil
	}
	key, err := cache.MetaNamespaceKeyFunc(exception)
	if err != nil {
		logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
		return engineapi.RuleError(rule.Name, ruleType, "failed to compute exception key", err)
	}

	level := podSecurityChecks.Level
	version := podSecurityChecks.Version
	checks := podSecurityChecks.Checks
	pod := podSecurityChecks.Pod
	levelVersion, _ := pss.ParseVersion(level, version)
	pssCheckResult := pss.ApplyPodSecurityExclusion(levelVersion, exception.Spec.PodSecurity, checks, &pod)

	if len(pssCheckResult) == 0 {
		logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
		return engineapi.RuleSkip(rule.Name, ruleType, "rule skipped due to policy exception "+key).WithException(exception)
	}
	return nil
}

// matchesException checks if an exception applies to the incoming resource.
// It returns the matched policy exception and the resource.
func matchesException(
	polexs []kyvernov2beta1.PolicyException,
	policyContext engineapi.PolicyContext,
) *kyvernov2beta1.PolicyException {
	gvk, subresource := policyContext.ResourceKind()
	resource := policyContext.NewResource()
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	for _, polex := range polexs {
		err := matched.CheckMatchesResources(
			resource,
			polex.Spec.Match,
			policyContext.NamespaceLabels(),
			policyContext.AdmissionInfo(),
			gvk,
			subresource,
		)
		// if there's no error it means a match
		if err == nil {
			return &polex
		}
	}
	return nil
}
