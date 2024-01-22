package webhook

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"golang.org/x/exp/maps"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	objectmeta "k8s.io/client-go/tools/cache"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on kind, failurePolicy and webhookTimeout
// a fine-grained webhook is created per policy with a unique path
type webhook struct {
	// policyMeta is set for fine-grained webhooks
	policyMeta objectmeta.ObjectName

	maxWebhookTimeout int32
	failurePolicy     admissionregistrationv1.FailurePolicyType
	rules             map[schema.GroupVersion]sets.Set[string]
	matchConditions   []admissionregistrationv1.MatchCondition
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             map[schema.GroupVersion]sets.Set[string]{},
		matchConditions:   matchConditions,
	}
}

func newWebhookPerPolicy(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition, policy kyvernov1.PolicyInterface) *webhook {
	webhook := newWebhook(timeout, failurePolicy, matchConditions)
	webhook.policyMeta = objectmeta.ObjectName{
		Namespace: policy.GetNamespace(),
		Name:      policy.GetName(),
	}
	if policy.GetSpec().CustomWebhookConfigurations() {
		webhook.matchConditions = policy.GetSpec().GetMatchConditions()
	}
	return webhook
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

func (wh *webhook) key(separator string) string {
	p := wh.policyMeta
	if p.Namespace != "" {
		return p.Namespace + separator + p.Name
	}
	return p.Name
}

func objectMeta(name string, annotations map[string]string, labels map[string]string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	desiredLabels := make(map[string]string)
	defaultLabels := map[string]string{
		kyverno.LabelWebhookManagedBy: kyverno.ValueKyvernoApp,
	}
	maps.Copy(desiredLabels, labels)
	maps.Copy(desiredLabels, defaultLabels)
	return metav1.ObjectMeta{
		Name:            name,
		Labels:          desiredLabels,
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

func webhookNameAndPath(wh webhook, baseName, basePath string) (name string, path string) {
	if wh.failurePolicy == ignore {
		name = baseName + "-ignore"
		path = basePath + "/ignore"
	} else {
		name = baseName + "-fail"
		path = basePath + "/fail"
	}
	if wh.policyMeta.Name != "" {
		name = name + "-fine-grained-" + wh.key("-")
		path = path + "/matchconditions/" + wh.key("/")
	}
	return name, path
}
