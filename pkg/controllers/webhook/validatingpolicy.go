package webhook

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(cfg config.Configuration, server, name, path string, servicePort int32, caBundle []byte, policies []engineapi.GenericPolicy) (webhooks []admissionregistrationv1.ValidatingWebhook) {
	var fineGrained, basic []engineapi.GenericPolicy
	for _, policy := range policies {
		var p policiesv1alpha1.GenericPolicy
		if vpol := policy.AsValidatingPolicy(); vpol != nil {
			p = vpol
		} else if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
			p = ivpol
		}
		if p.GetMatchConditions() != nil {
			fineGrained = append(fineGrained, policy)
		} else if p.GetMatchConstraints().MatchPolicy != nil && *p.GetMatchConstraints().MatchPolicy == admissionregistrationv1.Exact {
			fineGrained = append(fineGrained, policy)
		} else if p.GetWebhookConfiguration() != nil && p.GetWebhookConfiguration().TimeoutSeconds != nil {
			fineGrained = append(fineGrained, policy)
		} else {
			basic = append(basic, policy)
		}
	}
	// process fine grained policies
	var webhookIgnoreList, webhookFailList []admissionregistrationv1.ValidatingWebhook
	for _, policy := range basic {
		var p policiesv1alpha1.GenericPolicy
		if vpol := policy.AsValidatingPolicy(); vpol != nil {
			p = vpol
		} else if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
			p = ivpol
		}
		webhook := admissionregistrationv1.ValidatingWebhook{
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
		}
		for _, match := range p.GetMatchConstraints().ResourceRules {
			webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
		}
		if p.GetMatchConstraints().MatchPolicy != nil && *p.GetMatchConstraints().MatchPolicy == admissionregistrationv1.Exact {
			webhook.MatchPolicy = p.GetMatchConstraints().MatchPolicy
		}
		if p.GetWebhookConfiguration() != nil && p.GetWebhookConfiguration().TimeoutSeconds != nil {
			webhook.TimeoutSeconds = p.GetWebhookConfiguration().TimeoutSeconds
		}
		if p.GetFailurePolicy() == admissionregistrationv1.Ignore {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
			webhook.Name = name + "-ignore-finegrained-" + p.GetName()
			webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, path+"/ignore"+config.FineGrainedWebhookPath+"/"+p.GetName())
			webhookIgnoreList = append(webhookIgnoreList, webhook)
		} else {
			webhook.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
			webhook.Name = name + "-fail-finegrained-" + p.GetName()
			webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, path+"/fail"+config.FineGrainedWebhookPath+"/"+p.GetName())
			webhookFailList = append(webhookFailList, webhook)
		}
	}
	// process basic policies
	webhookIgnore := admissionregistrationv1.ValidatingWebhook{
		Name:                    name + "-ignore",
		ClientConfig:            newClientConfig(server, servicePort, caBundle, path+"/ignore"),
		FailurePolicy:           ptr.To(admissionregistrationv1.Ignore),
		SideEffects:             &noneOnDryRun,
		AdmissionReviewVersions: []string{"v1"},
	}
	webhookFail := admissionregistrationv1.ValidatingWebhook{
		Name:                    name + "-fail",
		ClientConfig:            newClientConfig(server, servicePort, caBundle, path+"/fail"),
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		SideEffects:             &noneOnDryRun,
		AdmissionReviewVersions: []string{"v1"},
	}
	if cfg.GetWebhook().NamespaceSelector != nil {
		webhookIgnore.NamespaceSelector = cfg.GetWebhook().NamespaceSelector
		webhookFail.NamespaceSelector = cfg.GetWebhook().NamespaceSelector
	}
	if cfg.GetWebhook().ObjectSelector != nil {
		webhookIgnore.ObjectSelector = cfg.GetWebhook().ObjectSelector
		webhookFail.ObjectSelector = cfg.GetWebhook().ObjectSelector
	}
	for _, policy := range basic {
		var p policiesv1alpha1.GenericPolicy
		if vpol := policy.AsValidatingPolicy(); vpol != nil {
			p = vpol
		} else if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
			p = ivpol
		}
		failurePolicyIgnore := p.GetFailurePolicy() == admissionregistrationv1.Ignore
		for _, match := range p.GetMatchConstraints().ResourceRules {
			if failurePolicyIgnore {
				webhookIgnore.Rules = append(webhookIgnore.Rules, match.RuleWithOperations)
			} else {
				webhookFail.Rules = append(webhookFail.Rules, match.RuleWithOperations)
			}
		}
	}

	// for _, pol := range policies {
	// 	if p.GetMatchConditions() != nil {
	// 		for _, m := range p.GetMatchConditions() {
	// 			if ok := autogen.CanAutoGen(matchResource); ok {
	// 				webhook.MatchConditions = append(webhook.MatchConditions, admissionregistrationv1.MatchCondition{
	// 					Name:       m.Name,
	// 					Expression: "!(object.kind == 'Pod') || " + m.Expression,
	// 				})
	// 			} else {
	// 				webhook.MatchConditions = p.GetMatchConditions()
	// 			}
	// 		}
	// 		fineGrainedWebhook = true
	// 	}

	// 	if vpol, ok := p.(*policiesv1alpha1.ValidatingPolicy); ok {
	// 		rules, _ := vpolautogen.Autogen(vpol)
	// 		for _, rule := range rules {
	// 			webhook.MatchConditions = append(webhook.MatchConditions, rule.Spec.MatchConditions...)
	// 			for _, match := range rule.Spec.MatchConstraints.ResourceRules {
	// 				webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
	// 			}
	// 		}
	// 	}

	// 	if ivpol, ok := p.(*policiesv1alpha1.ImageValidatingPolicy); ok {
	// 		autogeneratedIvPols, err := ivpolautogen.Autogen(ivpol)
	// 		if err != nil {
	// 			continue
	// 		}
	// 		for _, p := range autogeneratedIvPols {
	// 			webhook.MatchConditions = append(webhook.MatchConditions, p.Spec.MatchConditions...)
	// 			for _, match := range p.Spec.MatchConstraints.ResourceRules {
	// 				webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
	// 			}
	// 		}
	// 	}
	// }

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
