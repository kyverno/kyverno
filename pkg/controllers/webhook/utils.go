package webhook

import (
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils"
	"golang.org/x/exp/slices"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on kind, failurePolicy and webhookTimeout
type webhook struct {
	maxWebhookTimeout int32
	failurePolicy     admissionregistrationv1.FailurePolicyType
	rules             map[schema.GroupVersionResource]struct{}
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             map[schema.GroupVersionResource]struct{}{},
	}
}

func (wh *webhook) buildRulesWithOperations(ops ...admissionregistrationv1.OperationType) []admissionregistrationv1.RuleWithOperations {
	var rules []admissionregistrationv1.RuleWithOperations
	for gvr := range wh.rules {
		resources := sets.New(gvr.Resource)
		ephemeralContainersGVR := schema.GroupVersionResource{Resource: "pods/ephemeralcontainers", Group: "", Version: "v1"}
		_, rulesContainEphemeralContainers := wh.rules[ephemeralContainersGVR]
		if resources.Has("pods") && !rulesContainEphemeralContainers {
			resources.Insert("pods/ephemeralcontainers")
		}
		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gvr.Group},
				APIVersions: []string{gvr.Version},
				Resources:   sets.List(resources),
			},
			Operations: ops,
		})
	}
	less := func(a []string, b []string) (bool, bool) {
		if len(a) != len(b) {
			return len(a) < len(b), true
		}
		for i := range a {
			if a[i] != b[i] {
				return a[i] < b[i], true
			}
		}
		return false, false
	}
	slices.SortFunc(rules, func(a admissionregistrationv1.RuleWithOperations, b admissionregistrationv1.RuleWithOperations) bool {
		if less, match := less(a.APIGroups, b.APIGroups); match {
			return less
		}
		if less, match := less(a.APIVersions, b.APIVersions); match {
			return less
		}
		if less, match := less(a.Resources, b.Resources); match {
			return less
		}
		return false
	})
	return rules
}

func (wh *webhook) set(gvr schema.GroupVersionResource) {
	wh.rules[gvr] = struct{}{}
}

func (wh *webhook) isEmpty() bool {
	return len(wh.rules) == 0
}

func (wh *webhook) setWildcard() {
	wh.rules = map[schema.GroupVersionResource]struct{}{
		{Group: "*", Version: "*", Resource: "*/*"}: {},
	}
}

func hasWildcard(policies ...kyvernov1.PolicyInterface) bool {
	for _, policy := range policies {
		spec := policy.GetSpec()
		for _, rule := range spec.Rules {
			if kinds := rule.MatchResources.GetKinds(); slices.Contains(kinds, "*") {
				return true
			}
		}
	}
	return false
}

func objectMeta(name string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			utils.ManagedByLabel: kyvernov1.ValueKyvernoApp,
		},
		OwnerReferences: owner,
	}
}

func setRuleCount(rules []kyvernov1.Rule, status *kyvernov1.PolicyStatus) {
	validateCount, generateCount, mutateCount, verifyImagesCount := 0, 0, 0, 0
	for _, rule := range rules {
		if !strings.HasPrefix(rule.Name, "autogen-") {
			if rule.HasGenerate() {
				generateCount += 1
			}
			if rule.HasValidate() {
				validateCount += 1
			}
			if rule.HasMutate() {
				mutateCount += 1
			}
			if rule.HasVerifyImages() {
				verifyImagesCount += 1
			}
		}
	}
	status.RuleCount.Validate = validateCount
	status.RuleCount.Generate = generateCount
	status.RuleCount.Mutate = mutateCount
	status.RuleCount.VerifyImages = verifyImagesCount
}
