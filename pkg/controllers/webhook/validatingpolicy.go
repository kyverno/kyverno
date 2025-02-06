package webhook

import (
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(cfg config.Configuration, server string, servicePort int32, caBundle []byte, vpols []kyvernov2alpha1.GenericPolicy) (webhooks []admissionregistrationv1.ValidatingWebhook) {
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
		// TODO(shuting): exclude?
		for _, match := range vpol.GetMatchConstraints().ResourceRules {
			webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
		}

		fineGrainedWebhook := false
		if vpol.GetMatchConditions() != nil {
			webhook.MatchConditions = vpol.GetMatchConditions()
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

		if fineGrainedWebhook {
			webhook.SideEffects = &noneOnDryRun
			webhook.AdmissionReviewVersions = []string{"v1"}
			if failurePolicyIgnore {
				webhook.Name = config.ValidatingPolicyWebhookName + "-ignore-finegrained-" + vpol.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, "/validate/ignore"+config.FineGrainedWebhookPath+"/"+vpol.GetName())
				webhookIgnoreList = append(webhookIgnoreList, webhook)
			} else {
				webhook.Name = config.ValidatingPolicyWebhookName + "-fail-finegrained-" + vpol.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, "/validate/fail"+config.FineGrainedWebhookPath+"/"+vpol.GetName())
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
