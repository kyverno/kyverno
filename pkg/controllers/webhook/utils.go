package webhook

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	rules             map[schema.GroupVersion]sets.Set[string]
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             map[schema.GroupVersion]sets.Set[string]{},
	}
}

func (wh *webhook) buildRulesWithOperations(ops ...admissionregistrationv1.OperationType) []admissionregistrationv1.RuleWithOperations {
	var rules []admissionregistrationv1.RuleWithOperations
	for gv, resources := range wh.rules {
		// if we have pods, we add pods/ephemeralcontainers by default
		if (gv.Group == "" || gv.Group == "*") && (gv.Version == "v1" || gv.Version == "*") && (resources.Has("pods") || resources.Has("*")) {
			resources.Insert("pods/ephemeralcontainers")
		}
		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gv.Group},
				APIVersions: []string{gv.Version},
				Resources:   sets.List(resources),
			},
			Operations: ops,
		})
	}
	less := func(a []string, b []string) (int, bool) {
		if x := cmp.Compare(len(a), len(b)); x != 0 {
			return x, true
		}
		for i := range a {
			if x := cmp.Compare(a[i], b[i]); x != 0 {
				return x, true
			}
		}
		return 0, false
	}
	slices.SortFunc(rules, func(a admissionregistrationv1.RuleWithOperations, b admissionregistrationv1.RuleWithOperations) int {
		if x, match := less(a.APIGroups, b.APIGroups); match {
			return x
		}
		if x, match := less(a.APIVersions, b.APIVersions); match {
			return x
		}
		if x, match := less(a.Resources, b.Resources); match {
			return x
		}
		return 0
	})
	return rules
}

func (wh *webhook) set(gvrs schema.GroupVersionResource) {
	gv := gvrs.GroupVersion()
	resources := wh.rules[gv]
	if resources == nil {
		wh.rules[gv] = sets.New(gvrs.Resource)
	} else {
		resources.Insert(gvrs.Resource)
	}
}

func (wh *webhook) isEmpty() bool {
	return len(wh.rules) == 0
}

func objectMeta(name string, annotations map[string]string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			kyverno.LabelWebhookManagedBy: kyverno.ValueKyvernoApp,
		},
		Annotations:     annotations,
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

func capTimeout(maxWebhookTimeout int32) int32 {
	if maxWebhookTimeout > 30 {
		return 30
	}
	return maxWebhookTimeout
}
