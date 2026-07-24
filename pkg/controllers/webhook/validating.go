package webhook

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	ivpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/autogen"
	mpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func buildWebhookRules(cfg config.Configuration, server, name, queryPath string, servicePort int32, caBundle []byte, policies []engineapi.GenericPolicy, expressionCache *expressionCache) []admissionregistrationv1.ValidatingWebhook {
	var fineGrained, basic []engineapi.GenericPolicy
	for _, policy := range policies {
		p := extractGenericPolicy(policy)
		if validConditions(expressionCache, p.GetMatchConditions()) != nil {
			fineGrained = append(fineGrained, policy)
		} else if p.GetMatchConstraints().MatchPolicy != nil && *p.GetMatchConstraints().MatchPolicy == admissionregistrationv1.Exact {
			fineGrained = append(fineGrained, policy)
		} else if p.GetTimeoutSeconds() != nil {
			fineGrained = append(fineGrained, policy)
		} else {
			basic = append(basic, policy)
		}
	}
	slices.SortFunc(fineGrained, func(a, b engineapi.GenericPolicy) int {
		if x := cmp.Compare(a.GetNamespace(), b.GetNamespace()); x != 0 {
			return x
		}
		return cmp.Compare(a.GetName(), b.GetName())
	})
	var webhooks []admissionregistrationv1.ValidatingWebhook
	// process fine grained policies
	if len(fineGrained) != 0 {
		var fineGrainedIgnoreList, fineGrainedFailList []admissionregistrationv1.ValidatingWebhook
		for _, policy := range fineGrained {
			p := extractGenericPolicy(policy)
			webhook := admissionregistrationv1.ValidatingWebhook{
				SideEffects:             &noneOnDryRun,
				AdmissionReviewVersions: []string{"v1"},
			}
			if ok := autogen.CanAutoGen(ptr.To(p.GetMatchConstraints())); ok {
				webhook.MatchConditions = append(
					webhook.MatchConditions,
					autogen.CreateMatchConditions(
						"",
						[]policiesv1beta1.Target{{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
							Kind:     "Pod",
						}},
						validConditions(expressionCache, p.GetMatchConditions()),
					)...,
				)
			} else {
				webhook.MatchConditions = append(webhook.MatchConditions, validConditions(expressionCache, p.GetMatchConditions())...)
			}

			if policy.AsGeneratingPolicy() != nil || policy.AsNamespacedGeneratingPolicy() != nil {
				// all four operations including CONNECT are needed for generate.
				for _, match := range p.GetMatchConstraints().ResourceRules {
					rule := match.RuleWithOperations
					rule.Operations = []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
						admissionregistrationv1.Connect,
					}
					webhook.Rules = append(webhook.Rules, rule)
				}
			} else {
				for _, match := range p.GetMatchConstraints().ResourceRules {
					webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
				}
			}
			if vpol := policy.AsValidatingPolicyLike(); vpol != nil {
				// Skip autogen webhook registration for pod-controller kinds when the policy
				// has native VAP generation enabled — those kinds are governed by VAPs instead.
				if vpolTyped, ok := p.(*policiesv1beta1.ValidatingPolicy); ok && vpolTyped.Spec.GenerateValidatingAdmissionPolicyEnabled() {
					// autogen kinds are covered by VAPs; no webhook rules needed for them
				} else {
					policies, err := vpolautogen.Autogen(vpol)
					if err != nil {
						continue
					}
					for _, config := range slices.Sorted(maps.Keys(policies)) {
						policy := policies[config]
						webhook.MatchConditions = append(
							webhook.MatchConditions,
							autogen.CreateMatchConditions(config, policy.Targets, validConditions(expressionCache, policy.Spec.MatchConditions))...,
						)
						for _, match := range policy.Spec.MatchConstraints.ResourceRules {
							webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
						}
					}
				}
			}
			if ivpol, ok := p.(*policiesv1beta1.ImageValidatingPolicy); ok {
				policies, err := ivpolautogen.Autogen(ivpol)
				if err != nil {
					continue
				}
				for _, config := range slices.Sorted(maps.Keys(policies)) {
					policy := policies[config]
					webhook.MatchConditions = append(
						webhook.MatchConditions,
						autogen.CreateMatchConditions(config, policy.Targets, validConditions(expressionCache, policy.Spec.MatchConditions))...,
					)
					for _, match := range policy.Spec.MatchConstraints.ResourceRules {
						webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
					}
				}
			}

			if mpol := policy.AsMutatingPolicyLike(); mpol != nil {
				policies, err := mpolautogen.Autogen(mpol)
				if err != nil {
					logger.Error(err, "failed to auto-generate mutating policy", "policy", mpol.GetName())
					continue
				}
				for _, config := range slices.Sorted(maps.Keys(policies)) {
					autogenPolicy := policies[config]
					webhook.MatchConditions = append(
						webhook.MatchConditions,
						autogen.CreateMatchConditions(config, autogenPolicy.Targets, validConditions(expressionCache, autogenPolicy.Spec.MatchConditions))...,
					)
					for _, match := range autogenPolicy.Spec.MatchConstraints.ResourceRules {
						webhook.Rules = append(webhook.Rules, match.RuleWithOperations)
					}
				}
			}

			if p.GetMatchConstraints().MatchPolicy != nil && *p.GetMatchConstraints().MatchPolicy == admissionregistrationv1.Exact {
				webhook.MatchPolicy = p.GetMatchConstraints().MatchPolicy
			}
			if p.GetTimeoutSeconds() != nil {
				webhook.TimeoutSeconds = p.GetTimeoutSeconds()
			}
			if p.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()) == admissionregistrationv1.Ignore {
				webhook.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
				webhook.Name = generateName(name+"-ignore-finegrained", p)
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, path.Join(queryPath, p.GetName()))
				webhook.NamespaceSelector = resolveNamespaceSelector(p, cfg)
				webhook.ObjectSelector = mergeLabelSelectors(
					p.GetMatchConstraints().ObjectSelector,
					cfg.GetWebhook().ObjectSelector,
				)
				fineGrainedIgnoreList = append(fineGrainedIgnoreList, webhook)
			} else {
				webhook.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
				webhook.Name = generateName(name+"-fail-finegrained", p)
				webhook.ClientConfig = newClientConfig(server, servicePort, caBundle, path.Join(queryPath, p.GetName()))
				webhook.NamespaceSelector = resolveNamespaceSelector(p, cfg)
				webhook.ObjectSelector = mergeLabelSelectors(
					p.GetMatchConstraints().ObjectSelector,
					cfg.GetWebhook().ObjectSelector,
				)
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
	// process basic policies, grouped by the selectors they resolve to. A webhook carries a single
	// namespaceSelector/objectSelector pair, so policies resolving to different selectors cannot
	// share one: the last one processed would overwrite the selector and the others would silently
	// stop being called for their own namespaces. Policies that share selectors (the common case,
	// including everything that sets no selector at all) still share a webhook.
	for _, group := range groupBySelectors(basic, cfg) {
		basic := group.policies
		names := make([]string, 0, len(basic))
		for _, policy := range basic {
			names = append(names, policy.GetName())
		}
		slices.Sort(names)
		dynamicPath := path.Join(names...)
		webhookIgnore := admissionregistrationv1.ValidatingWebhook{
			Name:                    name + "-ignore" + group.suffix,
			ClientConfig:            newClientConfig(server, servicePort, caBundle, path.Join(queryPath, dynamicPath)),
			FailurePolicy:           ptr.To(admissionregistrationv1.Ignore),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
			NamespaceSelector:       group.namespaceSelector,
			ObjectSelector:          group.objectSelector,
		}
		webhookFail := admissionregistrationv1.ValidatingWebhook{
			Name:                    name + "-fail" + group.suffix,
			ClientConfig:            newClientConfig(server, servicePort, caBundle, path.Join(queryPath, dynamicPath)),
			FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1"},
			NamespaceSelector:       group.namespaceSelector,
			ObjectSelector:          group.objectSelector,
		}

		for _, policy := range basic {
			p := extractGenericPolicy(policy)
			var webhookRules []admissionregistrationv1.RuleWithOperations
			if vpol, ok := p.(*policiesv1beta1.ValidatingPolicy); ok {
				// When VAP generation is enabled, pod-controller kinds are governed by native
				// VAPs rather than Kyverno webhooks, so skip adding their autogen rules here.
				// NOTE: VAP reconciliation is asynchronous. A brief enforcement gap may exist
				// between this flag being set and the autogen VAPs becoming active. The gap is
				// bounded by controller reconcile latency and is inherent to this opt-in feature.
				if !vpol.Spec.GenerateValidatingAdmissionPolicyEnabled() {
					rules, err := vpolautogen.Autogen(vpol)
					if err != nil {
						continue
					}
					for _, rule := range rules {
						for _, match := range rule.Spec.MatchConstraints.ResourceRules {
							webhookRules = append(webhookRules, match.RuleWithOperations)
						}
					}
				}
			}
			if ivpol, ok := p.(*policiesv1beta1.ImageValidatingPolicy); ok {
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
			if nivpol, ok := p.(*policiesv1beta1.NamespacedImageValidatingPolicy); ok {
				autogeneratedNivPols, err := ivpolautogen.Autogen(nivpol)
				if err != nil {
					continue
				}
				for _, p := range autogeneratedNivPols {
					for _, match := range p.Spec.MatchConstraints.ResourceRules {
						webhookRules = append(webhookRules, match.RuleWithOperations)
					}
				}
			}
			if mpol := policy.AsMutatingPolicy(); mpol != nil {
				rules, err := mpolautogen.Autogen(mpol)
				if err != nil {
					continue
				}
				for _, rule := range rules {
					for _, match := range rule.Spec.MatchConstraints.ResourceRules {
						webhookRules = append(webhookRules, match.RuleWithOperations)
					}
				}
			}
			if nmpol := policy.AsNamespacedMutatingPolicy(); nmpol != nil {
				rules, err := mpolautogen.Autogen(nmpol)
				if err != nil {
					continue
				}
				for _, rule := range rules {
					for _, match := range rule.Spec.MatchConstraints.ResourceRules {
						webhookRules = append(webhookRules, match.RuleWithOperations)
					}
				}
			}
			if policy.AsGeneratingPolicy() != nil || policy.AsNamespacedGeneratingPolicy() != nil {
				// all four operations including CONNECT are needed for generate.
				for _, match := range p.GetMatchConstraints().ResourceRules {
					rule := match.RuleWithOperations
					rule.Operations = []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
						admissionregistrationv1.Connect,
					}
					webhookRules = append(webhookRules, rule)
				}
			} else {
				for _, match := range p.GetMatchConstraints().ResourceRules {
					webhookRules = append(webhookRules, match.RuleWithOperations)
				}
			}
			if p.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()) == admissionregistrationv1.Ignore {
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

// selectorGroup is a set of policies that resolve to the same namespace and object selectors and
// can therefore share a webhook.
type selectorGroup struct {
	namespaceSelector *metav1.LabelSelector
	objectSelector    *metav1.LabelSelector
	policies          []engineapi.GenericPolicy
	// suffix disambiguates webhook names when the policies do not all resolve to the same
	// selectors. It is empty when there is a single group, so the common case (policies that set
	// no selector at all) keeps its webhook name.
	suffix string
}

// groupBySelectors buckets policies by the selectors their webhook would carry, so policies with
// different selectors get their own webhook instead of overwriting each other's.
func groupBySelectors(policies []engineapi.GenericPolicy, cfg config.Configuration) []*selectorGroup {
	groups := map[string]*selectorGroup{}
	var keys []string
	for _, policy := range policies {
		p := extractGenericPolicy(policy)
		namespaceSelector := resolveNamespaceSelector(p, cfg)
		objectSelector := mergeLabelSelectors(p.GetMatchConstraints().ObjectSelector, cfg.GetWebhook().ObjectSelector)
		// the full digest is the group key: a truncated one could collide and merge two different
		// selectors into a single webhook, applying the wrong filter to some policies
		key := selectorKey(namespaceSelector, objectSelector)
		group, ok := groups[key]
		if !ok {
			group = &selectorGroup{namespaceSelector: namespaceSelector, objectSelector: objectSelector}
			groups[key] = group
			keys = append(keys, key)
		}
		group.policies = append(group.policies, policy)
	}
	slices.Sort(keys)
	result := make([]*selectorGroup, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		if len(keys) > 1 {
			group.suffix = "-" + key[:8]
		}
		result = append(result, group)
	}
	return result
}

// selectorKey returns a stable identifier for a pair of selectors. It is the full digest so two
// different selectors can never be grouped together, and it is derived from the selectors alone so
// a webhook name does not change when policies are added to or removed from its group. The name
// only carries a prefix of it, which keeps the webhook name well inside the 253 character limit
// the API server enforces no matter how long the policy or namespace names are.
func selectorKey(namespaceSelector, objectSelector *metav1.LabelSelector) string {
	serialized, err := json.Marshal([]*metav1.LabelSelector{namespaceSelector, objectSelector})
	if err != nil {
		serialized = []byte(fmt.Sprintf("%v%v", namespaceSelector, objectSelector))
	}
	sum := sha256.Sum256(serialized)
	return hex.EncodeToString(sum[:])
}

// resolveNamespaceSelector returns the namespace selector for a policy's webhook. A namespaced policy
// only applies to resources in its own namespace, so its selector is pinned to that namespace via the
// kubernetes.io/metadata.name label regardless of any namespaceSelector in its matchConstraints. A
// cluster-scoped policy keeps its configured namespaceSelector.
func resolveNamespaceSelector(p policiesv1beta1.GenericPolicy, cfg config.Configuration) *metav1.LabelSelector {
	if ns := p.GetNamespace(); ns != "" {
		nameSelector := &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "kubernetes.io/metadata.name",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{ns},
			}},
		}
		return mergeLabelSelectors(nameSelector, cfg.GetWebhook().NamespaceSelector)
	}
	return mergeLabelSelectors(p.GetMatchConstraints().NamespaceSelector, cfg.GetWebhook().NamespaceSelector)
}

func mergeLabelSelectors(a, b *metav1.LabelSelector) *metav1.LabelSelector {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	merged := &metav1.LabelSelector{}

	// copy a
	for k, v := range a.MatchLabels {
		if merged.MatchLabels == nil {
			merged.MatchLabels = map[string]string{}
		}
		merged.MatchLabels[k] = v
	}
	merged.MatchExpressions = append(merged.MatchExpressions, a.MatchExpressions...)

	// copy b
	for k, v := range b.MatchLabels {
		if merged.MatchLabels == nil {
			merged.MatchLabels = map[string]string{}
		}
		merged.MatchLabels[k] = v
	}
	merged.MatchExpressions = append(merged.MatchExpressions, b.MatchExpressions...)

	// nil out empty slices/maps so DeepEqual matches what the API server stores
	if len(merged.MatchLabels) == 0 {
		merged.MatchLabels = nil
	}
	if len(merged.MatchExpressions) == 0 {
		merged.MatchExpressions = nil
	}

	return merged
}

func validConditions(celExpressionCache *expressionCache, conditions []admissionregistrationv1.MatchCondition) []admissionregistrationv1.MatchCondition {
	if celExpressionCache == nil {
		return nil
	}
	valid, err := celExpressionCache.ValidateMatchConditions(conditions)
	if err != nil {
		logger.V(6).Info("skip building the webhook with Kubernetes unknown match conditions", "error", err.ToAggregate().Error())
	}
	if len(valid) == len(conditions) {
		return conditions
	}
	return nil
}

func generateName(name string, policy policiesv1beta1.GenericPolicy) string {
	if ns := policy.GetNamespace(); ns != "" {
		return name + "-" + ns + "-" + policy.GetName()
	}

	return name + "-" + policy.GetName()
}
