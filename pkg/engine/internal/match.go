package internal

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	celutils "github.com/kyverno/kyverno/pkg/utils/cel"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
)

func MatchPolicyContext(logger logr.Logger, client engineapi.Client, policyContext engineapi.PolicyContext, configuration config.Configuration) bool {
	policy := policyContext.Policy()
	old := policyContext.OldResource()
	new := policyContext.NewResource()
	if !checkNamespacedPolicy(policy, new, old) {
		logger.V(4).Info("policy namespace doesn't match resource namespace")
		return false
	}
	gvk, subresource := policyContext.ResourceKind()
	if !checkResourceFilters(configuration, gvk, subresource, new, old) {
		logger.V(4).Info("configuration resource filters doesn't match resource")
		return false
	}

	if policy.GetSpec().GetMatchConditions() != nil {
		if !checkMatchConditions(logger, policyContext, gvk, subresource) {
			logger.V(4).Info("webhookConfiguration.matchConditions doesn't match request")
			return false
		}
	}
	return true
}

func checkResourceFilters(configuration config.Configuration, gvk schema.GroupVersionKind, subresource string, resources ...unstructured.Unstructured) bool {
	for _, resource := range resources {
		if resource.Object != nil {
			// TODO: account for generate name here ?
			if configuration.ToFilter(gvk, subresource, resource.GetNamespace(), resource.GetName()) {
				return false
			}
		}
	}
	return true
}

func checkNamespacedPolicy(policy kyvernov1.PolicyInterface, resources ...unstructured.Unstructured) bool {
	if policy.IsNamespaced() {
		policyNamespace := policy.GetNamespace()
		if policyNamespace == "" {
			policyNamespace = "default"
		}
		for _, resource := range resources {
			if resource.Object != nil {
				resourceNamespace := resource.GetNamespace()
				if resourceNamespace != policyNamespace || resourceNamespace == "" {
					return false
				}
			}
		}
	}
	return true
}

func checkMatchConditions(logger logr.Logger, policyContext engineapi.PolicyContext, gvk schema.GroupVersionKind, subresource string) bool {
	policy := policyContext.Policy()
	old := policyContext.OldResource()
	new := policyContext.NewResource()
	new.SetGroupVersionKind(gvk)
	old.SetGroupVersionKind(gvk)
	gvr := schema.GroupVersionResource(policyContext.RequestResource())
	requestInfo := policyContext.AdmissionInfo().AdmissionUserInfo
	userInfo := NewUser(requestInfo.Username, requestInfo.UID, requestInfo.Groups)
	admissionAttributes := admission.NewAttributesRecord(new.DeepCopyObject(), old.DeepCopyObject(), gvk, new.GetNamespace(), new.GetName(), gvr, subresource, admission.Operation(policyContext.Operation()), nil, false, &userInfo)
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(gvk.GroupVersion())
	versionedAttr, err := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), admission.NewObjectInterfacesFromScheme(scheme))
	if err != nil {
		logger.Error(err, "error creating versioned attributes")
		return false
	}

	optionalVars := cel.OptionalVariableDeclarations{HasParams: false, HasAuthorizer: false}
	compiler, err := celutils.NewCompiler(nil, nil, policy.GetSpec().GetMatchConditions(), nil)
	if err != nil {
		logger.Error(err, "error creating composited compiler")
		return false
	}
	matchConditionFilter := compiler.CompileMatchExpressions(optionalVars)
	matcher := matchconditions.NewMatcher(matchConditionFilter, nil, policy.GetKind(), "", policy.GetName())
	result := matcher.Match(context.TODO(), versionedAttr, nil, nil)
	return result.Matches
}
