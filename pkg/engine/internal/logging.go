package internal

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func LoggerWithPolicyContext(logger logr.Logger, policyContext engineapi.PolicyContext) logr.Logger {
	logger = LoggerWithPolicy(logger, policyContext.Policy())
	logger = LoggerWithResource(logger, "new", policyContext.NewResource())
	logger = LoggerWithResource(logger, "old", policyContext.OldResource())
	return logger
}

func LoggerWithPolicy(logger logr.Logger, policy kyvernov1.PolicyInterface) logr.Logger {
	return logger.WithValues(
		"policy.name", policy.GetName(),
		"policy.namespace", policy.GetNamespace(),
		"policy.apply", policy.GetSpec().GetApplyRules(),
	)
}

func LoggerWithResource(logger logr.Logger, prefix string, resource unstructured.Unstructured) logr.Logger {
	if resource.Object == nil {
		return logger
	}
	return logger.WithValues(
		prefix+".kind", resource.GetKind(),
		prefix+".namespace", resource.GetNamespace(),
		prefix+".name", resource.GetName(),
	)
}

func LoggerWithRule(logger logr.Logger, rule kyvernov1.Rule) logr.Logger {
	return logger.WithValues("rule.name", rule.Name)
}
