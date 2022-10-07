package webhook

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on kind, failurePolicy and webhookTimeout
type webhook struct {
	maxWebhookTimeout int32
	failurePolicy     admissionregistrationv1.FailurePolicyType
	groups            sets.String
	versions          sets.String
	resources         sets.String
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		groups:            sets.NewString(),
		versions:          sets.NewString(),
		resources:         sets.NewString(),
	}
}

func (wh *webhook) buildRuleWithOperations(ops ...admissionregistrationv1.OperationType) admissionregistrationv1.RuleWithOperations {
	return admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   wh.groups.List(),
			APIVersions: wh.versions.List(),
			Resources:   wh.resources.List(),
		},
		Operations: ops,
	}
}

func (wh *webhook) isEmpty() bool {
	return wh.groups.Len() == 0 || wh.versions.Len() == 0 || wh.resources.Len() == 0
}

func (wh *webhook) setWildcard() {
	wh.groups = sets.NewString("*")
	wh.versions = sets.NewString("*")
	wh.resources = sets.NewString("*/*")
}

func hasWildcard(policies ...kyvernov1.PolicyInterface) bool {
	for _, policy := range policies {
		spec := policy.GetSpec()
		for _, rule := range spec.Rules {
			if kinds := rule.MatchResources.GetKinds(); utils.ContainsString(kinds, "*") {
				return true
			}
		}
	}
	return false
}

func objectMeta(name string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			managedByLabel: kyvernov1.ValueKyvernoApp,
		},
		OwnerReferences: owner,
	}
}
