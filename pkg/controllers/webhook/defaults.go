package webhook

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// defaultValidatingWebhooks applies API-server defaults to desired webhooks so
// they compare equal to unchanged configurations read from the informer.
func defaultValidatingWebhooks(webhooks []admissionregistrationv1.ValidatingWebhook) {
	for i := range webhooks {
		w := &webhooks[i]
		if w.FailurePolicy == nil {
			w.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}
		if w.MatchPolicy == nil {
			w.MatchPolicy = ptr.To(admissionregistrationv1.Equivalent)
		}
		w.NamespaceSelector = defaultWebhookSelector(w.NamespaceSelector)
		w.ObjectSelector = defaultWebhookSelector(w.ObjectSelector)
		if w.TimeoutSeconds == nil {
			w.TimeoutSeconds = ptr.To[int32](10)
		}
		defaultWebhookRulesAndService(w.Rules, w.ClientConfig.Service)
	}
}

// defaultMutatingWebhooks applies the same defaults as the validating path,
// plus the mutating webhook's Never reinvocation policy.
func defaultMutatingWebhooks(webhooks []admissionregistrationv1.MutatingWebhook) {
	for i := range webhooks {
		w := &webhooks[i]
		if w.FailurePolicy == nil {
			w.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}
		if w.MatchPolicy == nil {
			w.MatchPolicy = ptr.To(admissionregistrationv1.Equivalent)
		}
		w.NamespaceSelector = defaultWebhookSelector(w.NamespaceSelector)
		w.ObjectSelector = defaultWebhookSelector(w.ObjectSelector)
		if w.TimeoutSeconds == nil {
			w.TimeoutSeconds = ptr.To[int32](10)
		}
		if w.ReinvocationPolicy == nil {
			w.ReinvocationPolicy = ptr.To(admissionregistrationv1.NeverReinvocationPolicy)
		}
		defaultWebhookRulesAndService(w.Rules, w.ClientConfig.Service)
	}
}

// defaultWebhookSelector canonicalizes fields omitted by the API server's JSON
// round trip. It copies before clearing them because a built webhook may share
// its selector with an informer policy or the runtime configuration.
func defaultWebhookSelector(selector *metav1.LabelSelector) *metav1.LabelSelector {
	if selector == nil {
		return &metav1.LabelSelector{}
	}
	if selector.MatchLabels != nil && len(selector.MatchLabels) == 0 ||
		selector.MatchExpressions != nil && len(selector.MatchExpressions) == 0 {
		selector = selector.DeepCopy()
		if len(selector.MatchLabels) == 0 {
			selector.MatchLabels = nil
		}
		if len(selector.MatchExpressions) == 0 {
			selector.MatchExpressions = nil
		}
	}
	return selector
}

// defaultWebhookRulesAndService fills the API-server defaults for rule scope
// and service port while leaving explicitly configured values unchanged.
func defaultWebhookRulesAndService(rules []admissionregistrationv1.RuleWithOperations, service *admissionregistrationv1.ServiceReference) {
	for i := range rules {
		if rules[i].Scope == nil {
			rules[i].Scope = ptr.To(admissionregistrationv1.AllScopes)
		}
	}
	if service != nil && service.Port == nil {
		service.Port = ptr.To[int32](443)
	}
}
