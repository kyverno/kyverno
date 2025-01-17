package webhook

import (
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(server string, servicePort int32, caBundle []byte, vpols []*kyvernov2alpha1.ValidatingPolicy) (webhooks []admissionregistrationv1.ValidatingWebhook) {
	var (
		webhookIgnoreList []admissionregistrationv1.ValidatingWebhook
		webhookFailList   []admissionregistrationv1.ValidatingWebhook
		webhookIgnore     = admissionregistrationv1.ValidatingWebhook{
			Name:                    config.ValidatingPolicyWebhookName + "-ignore",
			ClientConfig:            newClientConfig(server, servicePort, caBundle, "/ignore"),
			FailurePolicy:           ptr.To(admissionregistrationv1.Ignore),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
		}
		webhookFail = admissionregistrationv1.ValidatingWebhook{
			Name:                    config.ValidatingPolicyWebhookName + "-fail",
			ClientConfig:            newClientConfig(server, servicePort, caBundle, "/fail"),
			FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
		}
	)
	for _, vpol := range vpols {
		webhook := admissionregistrationv1.ValidatingWebhook{}
		failurePolicyIgnore := vpol.Spec.FailurePolicy != nil && *vpol.Spec.FailurePolicy == admissionregistrationv1.Ignore
		if failurePolicyIgnore {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
		} else {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}
		// TODO(shuting): exclude?
		for _, match := range vpol.Spec.MatchConstraints.ResourceRules {
			webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
		}

		fineGrainedWebhook := false
		if vpol.Spec.MatchConditions != nil {
			webhook.MatchConditions = vpol.Spec.MatchConditions
			fineGrainedWebhook = true
		}
		if vpol.Spec.MatchConstraints.MatchPolicy != nil && *vpol.Spec.MatchConstraints.MatchPolicy == admissionregistrationv1.Exact {
			webhook.MatchPolicy = vpol.Spec.MatchConstraints.MatchPolicy
			fineGrainedWebhook = true
		}
		if vpol.Spec.WebhookConfiguration != nil && vpol.Spec.WebhookConfiguration.TimeoutSeconds != nil {
			webhook.TimeoutSeconds = vpol.Spec.WebhookConfiguration.TimeoutSeconds
			fineGrainedWebhook = true
		}

		if fineGrainedWebhook {
			webhook.SideEffects = &noneOnDryRun
			webhook.AdmissionReviewVersions = []string{"v1"}
			if failurePolicyIgnore {
				webhook.Name = config.ValidatingPolicyWebhookName + "-ignore-finegrained-" + vpol.Name
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, "/ignore"+config.FineGrainedWebhookPath+"/"+vpol.Name)
				webhookIgnoreList = append(webhookIgnoreList, webhook)
			} else {
				webhook.Name = config.ValidatingPolicyWebhookName + "-fail-finegrained-" + vpol.Name
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, "/fail"+config.FineGrainedWebhookPath+"/"+vpol.Name)
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
