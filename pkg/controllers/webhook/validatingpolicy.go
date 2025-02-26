package webhook

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(cfg config.Configuration, server string, servicePort int32, caBundle []byte, vpols []policiesv1alpha1.ValidatingPolicyInterface) (webhooks []admissionregistrationv1.ValidatingWebhook) {
	var (
		webhookIgnoreList []admissionregistrationv1.ValidatingWebhook
		webhookFailList   []admissionregistrationv1.ValidatingWebhook
		webhookIgnore     = admissionregistrationv1.ValidatingWebhook{
			Name:                    config.ValidatingPolicyWebhookName + "-ignore",
			ClientConfig:            newClientConfig(server, servicePort, caBundle, config.ValidatingPolicyServicePath+"/ignore"),
			FailurePolicy:           ptr.To(admissionregistrationv1.Ignore),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
		}
		webhookFail = admissionregistrationv1.ValidatingWebhook{
			Name:                    config.ValidatingPolicyWebhookName + "-fail",
			ClientConfig:            newClientConfig(server, servicePort, caBundle, config.ValidatingPolicyServicePath+"/fail"),
			FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
		}
	)

	if cfg.GetWebhook().NamespaceSelector != nil {
		webhookIgnore.NamespaceSelector = cfg.GetWebhook().NamespaceSelector
		webhookFail.NamespaceSelector = cfg.GetWebhook().NamespaceSelector
	}
	if cfg.GetWebhook().ObjectSelector != nil {
		webhookIgnore.ObjectSelector = cfg.GetWebhook().ObjectSelector
		webhookFail.ObjectSelector = cfg.GetWebhook().ObjectSelector
	}
	for _, vpol := range vpols {
		webhook := admissionregistrationv1.ValidatingWebhook{}
		failurePolicyIgnore := vpol.GetFailurePolicy() == admissionregistrationv1.Ignore
		if failurePolicyIgnore {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
		} else {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}

		for _, match := range vpol.GetMatchConstraints().ResourceRules {
			webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
		}

		fineGrainedWebhook := false
		if vpol.GetMatchConditions() != nil {
			for _, m := range vpol.GetMatchConditions() {
				if ok, _ := autogen.CanAutoGen(vpol.GetSpec()); ok {
					webhook.MatchConditions = append(webhook.MatchConditions, admissionregistrationv1.MatchCondition{
						Name:       m.Name,
						Expression: "!(object.kind == 'Pod') || " + m.Expression,
					})
				} else {
					webhook.MatchConditions = vpol.GetMatchConditions()
				}
			}
			fineGrainedWebhook = true
		}
		if vpol.GetMatchConstraints().MatchPolicy != nil && *vpol.GetMatchConstraints().MatchPolicy == admissionregistrationv1.Exact {
			webhook.MatchPolicy = vpol.GetMatchConstraints().MatchPolicy
			fineGrainedWebhook = true
		}
		if vpol.GetWebhookConfiguration() != nil && vpol.GetWebhookConfiguration().TimeoutSeconds != nil {
			webhook.TimeoutSeconds = vpol.GetWebhookConfiguration().TimeoutSeconds
			fineGrainedWebhook = true
		}

		for _, rule := range autogen.ComputeRules(vpol.(*policiesv1alpha1.ValidatingPolicy)) {
			webhook.MatchConditions = append(webhook.MatchConditions, rule.MatchConditions...)
			for _, match := range rule.MatchConstraints.ResourceRules {
				webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
			}
		}

		if fineGrainedWebhook {
			webhook.SideEffects = &noneOnDryRun
			webhook.AdmissionReviewVersions = []string{"v1"}
			if failurePolicyIgnore {
				webhook.Name = config.ValidatingPolicyWebhookName + "-ignore-finegrained-" + vpol.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, "/vpol/ignore"+config.FineGrainedWebhookPath+"/"+vpol.GetName())
				webhookIgnoreList = append(webhookIgnoreList, webhook)
			} else {
				webhook.Name = config.ValidatingPolicyWebhookName + "-fail-finegrained-" + vpol.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, "/vpol/fail"+config.FineGrainedWebhookPath+"/"+vpol.GetName())
				webhookFailList = append(webhookFailList, webhook)
			}
		} else {
			if failurePolicyIgnore {
				webhookIgnore.Rules = append(webhookIgnore.Rules, webhook.Rules...)
			} else {
				webhookFail.Rules = append(webhookFail.Rules, webhook.Rules...)
			}
		}
	}

	if webhookFailList != nil {
		webhooks = append(webhooks, webhookFailList...)
	}
	if webhookIgnoreList != nil {
		webhooks = append(webhooks, webhookIgnoreList...)
	}
	if webhookFail.Rules != nil {
		webhooks = append(webhooks, webhookFail)
	}
	if webhookIgnore.Rules != nil {
		webhooks = append(webhooks, webhookIgnore)
	}
	return
}
