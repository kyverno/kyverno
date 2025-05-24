package webhook

import (
	"maps"
	"path"
	"slices"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	ivpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/autogen"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(cfg config.Configuration, server, name, queryPath string, servicePort int32, caBundle []byte, policies []engineapi.GenericPolicy) []admissionregistrationv1.ValidatingWebhook {
	var fineGrained, basic []engineapi.GenericPolicy
	for _, policy := range policies {
		var p policiesv1alpha1.GenericPolicy
		if vpol := policy.AsValidatingPolicy(); vpol != nil {
			p = vpol
		} else if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
			p = ivpol
		} else if gpol := policy.AsGeneratingPolicy(); gpol != nil {
			p = gpol
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
	var webhooks []admissionregistrationv1.ValidatingWebhook
	// process fine grained policies
	if len(fineGrained) != 0 {
		var fineGrainedIgnoreList, fineGrainedFailList []admissionregistrationv1.ValidatingWebhook
		for _, policy := range fineGrained {
			var p policiesv1alpha1.GenericPolicy
			if vpol := policy.AsValidatingPolicy(); vpol != nil {
				p = vpol
			} else if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
				p = ivpol
			} else if gpol := policy.AsGeneratingPolicy(); gpol != nil {
				p = gpol
			}
			webhook := admissionregistrationv1.ValidatingWebhook{
				SideEffects:             &noneOnDryRun,
				AdmissionReviewVersions: []string{"v1"},
			}
			if ok := autogen.CanAutoGen(ptr.To(p.GetMatchConstraints())); ok {
				webhook.MatchConditions = append(
					webhook.MatchConditions,
					autogen.CreateMatchConditions(
						"",
						[]policiesv1alpha1.Target{{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
							Kind:     "Pod",
						}},
						p.GetMatchConditions(),
					)...,
				)
			} else {
				webhook.MatchConditions = append(webhook.MatchConditions, p.GetMatchConditions()...)
			}
			for _, match := range p.GetMatchConstraints().ResourceRules {
				webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
			}
			if vpol, ok := p.(*policiesv1alpha1.ValidatingPolicy); ok {
				policies, err := vpolautogen.Autogen(vpol)
				if err != nil {
					continue
				}
				for _, config := range slices.Sorted(maps.Keys(policies)) {
					policy := policies[config]
					webhook.MatchConditions = append(
						webhook.MatchConditions,
						autogen.CreateMatchConditions(config, policy.Targets, policy.Spec.MatchConditions)...,
					)
					for _, match := range policy.Spec.MatchConstraints.ResourceRules {
						webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
					}
				}
			}
			if ivpol, ok := p.(*policiesv1alpha1.ImageValidatingPolicy); ok {
				policies, err := ivpolautogen.Autogen(ivpol)
				if err != nil {
					continue
				}
				for _, config := range slices.Sorted(maps.Keys(policies)) {
					policy := policies[config]
					webhook.MatchConditions = append(
						webhook.MatchConditions,
						autogen.CreateMatchConditions(config, policy.Targets, policy.Spec.MatchConditions)...,
					)
					for _, match := range policy.Spec.MatchConstraints.ResourceRules {
						webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
					}
				}
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
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, path.Join(queryPath, p.GetName()))
				fineGrainedIgnoreList = append(fineGrainedIgnoreList, webhook)
			} else {
				webhook.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
				webhook.Name = name + "-fail-finegrained-" + p.GetName()
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, path.Join(queryPath, p.GetName()))
				fineGrainedFailList = append(fineGrainedFailList, webhook)
			}
		}
		if fineGrainedFailList != nil {
			webhooks = append(webhooks, fineGrainedFailList...)
		}
		if fineGrainedIgnoreList != nil {
			webhooks = append(webhooks, fineGrainedIgnoreList...)
		}
	}
	// process basic policies
	if len(basic) != 0 {
		names := make([]string, 0, len(basic))
		for _, policy := range basic {
			names = append(names, policy.GetName())
		}
		slices.Sort(names)
		dynamicPath := path.Join(names...)
		webhookIgnore := admissionregistrationv1.ValidatingWebhook{
			Name:                    name + "-ignore",
			ClientConfig:            newClientConfig(server, servicePort, caBundle, path.Join(queryPath, dynamicPath)),
			FailurePolicy:           ptr.To(admissionregistrationv1.Ignore),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
		}
		webhookFail := admissionregistrationv1.ValidatingWebhook{
			Name:                    name + "-fail",
			ClientConfig:            newClientConfig(server, servicePort, caBundle, path.Join(queryPath, dynamicPath)),
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
			} else if gpol := policy.AsGeneratingPolicy(); gpol != nil {
				p = gpol
			}
			var webhookRules []admissionregistrationv1.RuleWithOperations
			if vpol, ok := p.(*policiesv1alpha1.ValidatingPolicy); ok {
				rules, _ := vpolautogen.Autogen(vpol)
				for _, rule := range rules {
					for _, match := range rule.Spec.MatchConstraints.ResourceRules {
						webhookRules = append(webhookRules, match.RuleWithOperations)
					}
				}
			}
			if ivpol, ok := p.(*policiesv1alpha1.ImageValidatingPolicy); ok {
				autogeneratedIvPols, err := ivpolautogen.Autogen(ivpol)
				if err != nil {
					continue
				}
				for _, p := range autogeneratedIvPols {
					for _, match := range p.Spec.MatchConstraints.ResourceRules {
						webhookRules = append(webhookRules, match.RuleWithOperations)
					}
				}
			}
			for _, match := range p.GetMatchConstraints().ResourceRules {
				webhookRules = append(webhookRules, match.RuleWithOperations)
			}
			if p.GetFailurePolicy() == admissionregistrationv1.Ignore {
				webhookIgnore.Rules = append(webhookIgnore.Rules, webhookRules...)
			} else {
				webhookFail.Rules = append(webhookFail.Rules, webhookRules...)
			}
		}
		if webhookFail.Rules != nil {
			webhooks = append(webhooks, webhookFail)
		}
		if webhookIgnore.Rules != nil {
			webhooks = append(webhooks, webhookIgnore)
		}
	}
	return webhooks
}
