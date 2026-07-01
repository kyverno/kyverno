package webhook

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

const bootstrapExclusionMatchConditionName = "kyverno-exclude-bootstrap-resources"

// bootstrapExclusionExpression skips admission for Node and
// CertificateSigningRequest. These are cluster-scoped, so a namespaceSelector
// cannot exclude them, and they are required for a node to register. A persisted
// Fail webhook matching them blocks node registration while Kyverno is
// unavailable (for example after a full cluster stop/start), and the controller
// cannot self-recover because its own pods cannot be scheduled. The API server
// evaluates the expression, so it takes effect with no running Kyverno pod.
const bootstrapExclusionExpression = `!(request.resource.group == "" && request.resource.resource == "nodes") && !(request.resource.group == "certificates.k8s.io" && request.resource.resource == "certificatesigningrequests")`

// bootstrapExclusionMatchConditions returns the match condition that excludes
// bootstrap resources, or nil when the feature is disabled.
func bootstrapExclusionMatchConditions(exclude bool) []admissionregistrationv1.MatchCondition {
	if !exclude {
		return nil
	}
	return []admissionregistrationv1.MatchCondition{{
		Name:       bootstrapExclusionMatchConditionName,
		Expression: bootstrapExclusionExpression,
	}}
}

// excludeBootstrapResourcesFromValidatingWebhooks appends the bootstrap exclusion
// to every Fail webhook. Ignore webhooks already fail open and cannot deadlock,
// so they are left untouched. A nil FailurePolicy is treated as Fail to match the
// API server default.
func excludeBootstrapResourcesFromValidatingWebhooks(webhooks []admissionregistrationv1.ValidatingWebhook, exclude bool) {
	conditions := bootstrapExclusionMatchConditions(exclude)
	if conditions == nil {
		return
	}
	for i := range webhooks {
		if webhooks[i].FailurePolicy == nil || *webhooks[i].FailurePolicy == admissionregistrationv1.Fail {
			webhooks[i].MatchConditions = append(webhooks[i].MatchConditions, conditions...)
		}
	}
}

func excludeBootstrapResourcesFromMutatingWebhooks(webhooks []admissionregistrationv1.MutatingWebhook, exclude bool) {
	conditions := bootstrapExclusionMatchConditions(exclude)
	if conditions == nil {
		return
	}
	for i := range webhooks {
		if webhooks[i].FailurePolicy == nil || *webhooks[i].FailurePolicy == admissionregistrationv1.Fail {
			webhooks[i].MatchConditions = append(webhooks[i].MatchConditions, conditions...)
		}
	}
}
