package webhook

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(cfg config.Configuration, server string, servicePort int32, caBundle []byte, policies []engineapi.GenericPolicy) (webhooks []admissionregistrationv1.ValidatingWebhook) {
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
	for _, pol := range policies {
		var p policiesv1alpha1.GenericPolicy
		matchResource := &admissionregistrationv1.MatchResources{}
		if vpol := pol.AsValidatingPolicy(); vpol != nil {
			p = vpol
			matchResource = vpol.Spec.MatchConstraints
		} else if ivpol := pol.AsImageVerificationPolicy(); ivpol != nil {
			p = ivpol
		}

		webhook := admissionregistrationv1.ValidatingWebhook{}
		failurePolicyIgnore := p.GetFailurePolicy() == admissionregistrationv1.Ignore
		if failurePolicyIgnore {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
		} else {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
		}

		for _, match := range p.GetMatchConstraints().ResourceRules {
			webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
		}

		fineGrainedWebhook := false
		if p.GetMatchConditions() != nil {
			for _, m := range p.GetMatchConditions() {
				if ok, _ := autogen.CanAutoGen(matchResource); ok {
					webhook.MatchConditions = append(webhook.MatchConditions, admissionregistrationv1.MatchCondition{
						Name:       m.Name,
						Expression: "!(object.kind == 'Pod') || " + m.Expression,
					})
				} else {
					webhook.MatchConditions = p.GetMatchConditions()
				}
			}
			fineGrainedWebhook = true
		}
		if p.GetMatchConstraints().MatchPolicy != nil && *p.GetMatchConstraints().MatchPolicy == admissionregistrationv1.Exact {
			webhook.MatchPolicy = p.GetMatchConstraints().MatchPolicy
			fineGrainedWebhook = true
		}
		if p.GetWebhookConfiguration() != nil && p.GetWebhookConfiguration().TimeoutSeconds != nil {
			webhook.TimeoutSeconds = p.GetWebhookConfiguration().TimeoutSeconds
			fineGrainedWebhook = true
		}

		if vpol, ok := p.(*policiesv1alpha1.ValidatingPolicy); ok {
			for _, rule := range autogen.ComputeRules(vpol) {
				webhook.MatchConditions = append(webhook.MatchConditions, rule.MatchConditions...)
				for _, match := range rule.MatchConstraints.ResourceRules {
					webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
				}
			}
		}

		if fineGrainedWebhook {
			webhook.SideEffects = &noneOnDryRun
			webhook.AdmissionReviewVersions = []string{"v1"}
			if failurePolicyIgnore {
				webhook.Name = config.ValidatingPolicyWebhookName + "-ignore-finegrained-" + p.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, config.ValidatingPolicyServicePath+"/ignore"+config.FineGrainedWebhookPath+"/"+p.GetName())
				webhookIgnoreList = append(webhookIgnoreList, webhook)
			} else {
				webhook.Name = config.ValidatingPolicyWebhookName + "-fail-finegrained-" + p.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, config.ValidatingPolicyServicePath+"/fail"+config.FineGrainedWebhookPath+"/"+p.GetName())
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
