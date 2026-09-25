package webhook

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// Match the API server's admissionregistration/v1 defaults before comparing the
// desired webhooks with the informer copy. Otherwise, each watchdog tick sends
// an update even when the webhook configuration has not changed.
func defaultValidatingWebhooks(webhooks []admissionregistrationv1.ValidatingWebhook) {
	for i := range webhooks {
		w := &webhooks[i]
		if w.FailurePolicy == nil {
			w.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}
		if w.MatchPolicy == nil {
			w.MatchPolicy = ptr.To(admissionregistrationv1.Equivalent)
		}
		if w.NamespaceSelector == nil {
			w.NamespaceSelector = &metav1.LabelSelector{}
		}
		if w.ObjectSelector == nil {
			w.ObjectSelector = &metav1.LabelSelector{}
		}
		if w.TimeoutSeconds == nil {
			w.TimeoutSeconds = ptr.To[int32](10)
		}
		defaultWebhookRulesAndService(w.Rules, w.ClientConfig.Service)
	}
}

func defaultMutatingWebhooks(webhooks []admissionregistrationv1.MutatingWebhook) {
	for i := range webhooks {
		w := &webhooks[i]
		if w.FailurePolicy == nil {
			w.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}
		if w.MatchPolicy == nil {
			w.MatchPolicy = ptr.To(admissionregistrationv1.Equivalent)
		}
		if w.NamespaceSelector == nil {
			w.NamespaceSelector = &metav1.LabelSelector{}
		}
		if w.ObjectSelector == nil {
			w.ObjectSelector = &metav1.LabelSelector{}
		}
		if w.TimeoutSeconds == nil {
			w.TimeoutSeconds = ptr.To[int32](10)
		}
		if w.ReinvocationPolicy == nil {
			w.ReinvocationPolicy = ptr.To(admissionregistrationv1.NeverReinvocationPolicy)
		}
		defaultWebhookRulesAndService(w.Rules, w.ClientConfig.Service)
	}
}

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
