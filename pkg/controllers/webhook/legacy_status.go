package webhook

import (
	"context"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/deprecations"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

// This file holds Kyverno's own legacy ClusterPolicy/Policy status writer, moved here verbatim so
// its allowance to write the /status subresource can be registered, and removed, alongside the
// writer itself: see deprecations.AllowSubresourceForLegacyStatusWriter and decision 4 of the
// #17708 design. Deleting this file when #17710 removes legacy execution removes both the writer
// and its registration in one commit; there is no ordering discipline to maintain separately.
//
//nolint:gochecknoinits // registers this binary's legacy status-subresource allowance; see AGENTS.md precedent.
func init() {
	deprecations.AllowSubresourceForLegacyStatusWriter("status")
}

func (c *controller) updatePolicyStatuses(ctx context.Context, webhookType string) error {
	// While webhook health is unknown/unhealthy (startup, leader change, a cluster
	// resumed after an outage) the recorded webhook state has not been rebuilt from a
	// confirmed-healthy reconcile. Do not downgrade policies to NotReady in that
	// window: it evicts them from the policy cache and the handler then admits
	// requests unmutated/unvalidated, silently skipping failurePolicy: Fail rules
	// (#11560, #16281). Preserve the last known status; a healthy reconcile updates it.
	if !c.watchdogCheck() {
		return nil
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	policies, err := c.getAllPolicies()
	if err != nil {
		return err
	}
	updateStatusFunc := func(policy kyvernov1.PolicyInterface) error {
		policyKey, err := cache.MetaNamespaceKeyFunc(policy)
		if err != nil {
			return err
		}

		spec := policy.GetSpec()
		if webhookType == config.MutatingWebhookConfigurationName {
			if !(spec.HasMutateStandard() || spec.HasVerifyImages()) {
				return nil
			}
		} else if webhookType == config.ValidatingWebhookConfigurationName {
			if !(spec.HasValidate() || spec.HasGenerate() || spec.HasMutateExisting() || spec.HasVerifyImageChecks() || spec.HasVerifyManifests()) {
				return nil
			}
		}

		ready, message := true, "Ready"
		if c.autoUpdateWebhooks {
			if set, ok := c.policyState[webhookType]; ok {
				if !set.Has(policyKey) {
					ready, message = false, "Not Ready"
				}
			}
		}
		status := policy.GetStatus()
		status.SetReady(ready, message)
		status.Autogen.Rules = nil
		rules := autogen.Default.ComputeRules(policy, "")
		setRuleCount(rules, status)
		for _, rule := range rules {
			if strings.HasPrefix(rule.Name, "autogen-") {
				status.Autogen.Rules = append(status.Autogen.Rules, rule)
			}
		}
		return nil
	}
	for _, policy := range policies {
		if policy.GetNamespace() == "" {
			err := controllerutils.UpdateStatus(
				ctx,
				policy.(*kyvernov1.ClusterPolicy),
				c.kyvernoClient.KyvernoV1().ClusterPolicies(),
				func(policy *kyvernov1.ClusterPolicy) error {
					return updateStatusFunc(policy)
				},
				func(a *kyvernov1.ClusterPolicy, b *kyvernov1.ClusterPolicy) bool {
					return datautils.DeepEqual(a.Status, b.Status)
				},
			)
			if err != nil {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					objNew, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().Get(ctx, policy.GetName(), metav1.GetOptions{})
					if err != nil {
						return err
					}
					return controllerutils.UpdateStatus(
						ctx,
						objNew,
						c.kyvernoClient.KyvernoV1().ClusterPolicies(),
						func(policy *kyvernov1.ClusterPolicy) error {
							return updateStatusFunc(policy)
						},
						func(a *kyvernov1.ClusterPolicy, b *kyvernov1.ClusterPolicy) bool {
							return datautils.DeepEqual(a.Status, b.Status)
						},
					)
				})
				if retryErr != nil {
					logger.Error(retryErr, "failed to update clusterpolicy status", "policy", policy.GetName())
					continue
				}
			}
		} else {
			err := controllerutils.UpdateStatus(
				ctx,
				policy.(*kyvernov1.Policy),
				c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()),
				func(policy *kyvernov1.Policy) error {
					return updateStatusFunc(policy)
				},
				func(a *kyvernov1.Policy, b *kyvernov1.Policy) bool {
					return datautils.DeepEqual(a.Status, b.Status)
				},
			)
			if err != nil {
				retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					objNew, err := c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()).Get(ctx, policy.GetName(), metav1.GetOptions{})
					if err != nil {
						return err
					}
					return controllerutils.UpdateStatus(
						ctx,
						objNew,
						c.kyvernoClient.KyvernoV1().Policies(policy.GetNamespace()),
						func(policy *kyvernov1.Policy) error {
							return updateStatusFunc(policy)
						},
						func(a *kyvernov1.Policy, b *kyvernov1.Policy) bool {
							return datautils.DeepEqual(a.Status, b.Status)
						},
					)
				})
				if retryErr != nil {
					logger.Error(retryErr, "failed to update policy status", "namespace", policy.GetNamespace(), "policy", policy.GetName())
					continue
				}
			}
		}
	}
	return nil
}
