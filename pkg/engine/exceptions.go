package engine

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	matched "github.com/kyverno/kyverno/pkg/utils/match"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	polexs, err := selector.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var result []*kyvernov2alpha1.PolicyException
	policyName, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to compute policy key: %w", err)
	}
	for _, polex := range polexs {
		if polex.Contains(policyName, rule) {
			result = append(result, polex)
		}
	}
	return result, nil
}

// matchesException checks if an exception applies to the resource being admitted
func matchesException(
	selector engineapi.PolicyExceptionSelector,
	policyContext engineapi.PolicyContext,
	rule *kyvernov1.Rule,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
	cfg config.Configuration,
) (*kyvernov2alpha1.PolicyException, error) {
	candidates, err := findExceptions(selector, policyContext.Policy(), rule.Name)
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		err := matched.CheckMatchesResources(
			policyContext.NewResource(),
			candidate.Spec.Match,
			policyContext.NamespaceLabels(),
			subresourceGVKToAPIResource,
			policyContext.SubResource(),
			policyContext.AdmissionInfo(),
			cfg.GetExcludeGroupRole(),
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
func hasPolicyExceptions(
	log logr.Logger,
	selector engineapi.PolicyExceptionSelector,
	ctx engineapi.PolicyContext,
	rule *kyvernov1.Rule,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
	cfg config.Configuration,
) *engineapi.RuleResponse {
	// if matches, check if there is a corresponding policy exception
	exception, err := matchesException(selector, ctx, rule, subresourceGVKToAPIResource, cfg)
	// if we found an exception
	if err == nil && exception != nil {
		key, err := cache.MetaNamespaceKeyFunc(exception)
		if err != nil {
			log.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
			return &engineapi.RuleResponse{
				Name:    rule.Name,
				Message: "failed to find matched exception " + key,
				Status:  engineapi.RuleStatusError,
			}
		}
		log.V(3).Info("policy rule skipped due to policy exception", "exception", key)
		return &engineapi.RuleResponse{
			Name:    rule.Name,
			Message: "rule skipped due to policy exception " + key,
			Status:  engineapi.RuleStatusSkip,
		}
	}
	return nil
}
