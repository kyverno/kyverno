package webhook

import (
	"cmp"
	"slices"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"golang.org/x/exp/maps"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	objectmeta "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on group, kind, scopeType, failurePolicy and webhookTimeout
// a fine-grained webhook is created per policy with a unique path
type webhook struct {
	// policyMeta is set for fine-grained webhooks
	policyMeta        objectmeta.ObjectName
	maxWebhookTimeout int32
	failurePolicy     admissionregistrationv1.FailurePolicyType
	rules             sets.Set[ruleEntry]
	matchConditions   []admissionregistrationv1.MatchCondition
}

type ruleEntry struct {
	group       string
	version     string
	resource    string
	subresource string
	scope       admissionregistrationv1.ScopeType
	operation   admissionregistrationv1.OperationType
}

type aggregatedRuleEntry struct {
	group       string
	version     string
	resource    string
	subresource string
	scope       admissionregistrationv1.ScopeType
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             sets.New[ruleEntry](),
		matchConditions:   matchConditions,
	}
}

// func findKeyContainingSubstring(m map[string][]admissionregistrationv1.OperationType, substring string, defaultOpn []admissionregistrationv1.OperationType) []admissionregistrationv1.OperationType {
// 	for key, value := range m {
// 		if key == "Pod/exec" || strings.Contains(strings.ToLower(key), strings.ToLower(substring)) || strings.Contains(strings.ToLower(substring), strings.ToLower(key)) {
// 			return value
// 		}
// 	}
// 	return defaultOpn
// }

func newWebhookPerPolicy(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition, policy kyvernov1.PolicyInterface) *webhook {
	webhook := newWebhook(timeout, failurePolicy, matchConditions)
	webhook.policyMeta = objectmeta.ObjectName{
		Namespace: policy.GetNamespace(),
		Name:      policy.GetName(),
	}
	if policy.GetSpec().CustomWebhookMatchConditions() {
		webhook.matchConditions = policy.GetSpec().GetMatchConditions()
	}
	return webhook
}

func (wh *webhook) hasRule(
	group, version, resource, subresource string,
	scope admissionregistrationv1.ScopeType,
	operation admissionregistrationv1.OperationType,
) bool {
	var groups, versions, resources, subresources []string
	var scopes []admissionregistrationv1.ScopeType
	var operations []admissionregistrationv1.OperationType
	if group == "*" {
		groups = []string{group}
	} else {
		groups = []string{group, "*"}
	}
	if version == "*" {
		versions = []string{version}
	} else {
		versions = []string{version, "*"}
	}
	if resource == "*" {
		resources = []string{resource}
	} else {
		resources = []string{resource, "*"}
	}
	// TODO: probably a bit more couple with resource
	if subresource == "*" {
		subresources = []string{subresource}
	} else {
		subresources = []string{subresource, "*"}
	}
	if scope == admissionregistrationv1.AllScopes {
		scopes = []admissionregistrationv1.ScopeType{scope}
	} else {
		scopes = []admissionregistrationv1.ScopeType{scope, admissionregistrationv1.AllScopes}
	}
	if operation == admissionregistrationv1.OperationAll {
		operations = []admissionregistrationv1.OperationType{operation}
	} else {
		operations = []admissionregistrationv1.OperationType{operation, admissionregistrationv1.OperationAll}
	}
	for _, _scope := range scopes {
		for _, _group := range groups {
			for _, _version := range versions {
				for _, _resource := range resources {
					for _, _subresource := range subresources {
						for _, _operation := range operations {
							if _scope != scope || _group != group || _version != version || _resource != resource || _subresource != subresource || _operation != operation {
								test := ruleEntry{
									group:       _group,
									version:     _version,
									resource:    _resource,
									subresource: _subresource,
									scope:       _scope,
									operation:   _operation,
								}
								if wh.rules.Has(test) {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

func (wh *webhook) buildRulesWithOperations() []admissionregistrationv1.RuleWithOperations {
	rules := map[aggregatedRuleEntry]sets.Set[admissionregistrationv1.OperationType]{}
	// keep only the relevant rules
	for rule := range wh.rules {
		if !wh.hasRule(rule.group, rule.version, rule.resource, rule.subresource, rule.scope, rule.operation) {
			key := aggregatedRuleEntry{rule.group, rule.version, rule.resource, rule.subresource, rule.scope}
			ops := rules[key]
			if ops == nil {
				ops = sets.New[admissionregistrationv1.OperationType]()
			}
			ops.Insert(rule.operation)
			rules[key] = ops
		}
	}
	// build rules
	out := make([]admissionregistrationv1.RuleWithOperations, 0, len(rules))
	for rule, ops := range rules {
		resource := rule.resource
		if rule.subresource != "" {
			resource = rule.resource + "/" + rule.subresource
		}
		resources := []string{resource}
		// if we have pods, we add pods/ephemeralcontainers by default
		if (rule.group == "" || rule.group == "*") && (rule.version == "v1" || rule.version == "*") && (resource == "pods" || resource == "*") {
			resources = append(resources, "pods/ephemeralcontainers")
		}
		out = append(out, admissionregistrationv1.RuleWithOperations{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{rule.group},
				APIVersions: []string{rule.version},
				Resources:   resources,
				Scope:       ptr.To(rule.scope),
			},
			Operations: sets.List(ops),
		})
	}
	// sort rules
	for _, rule := range out {
		slices.Sort(rule.APIGroups)
		slices.Sort(rule.APIVersions)
		slices.Sort(rule.Resources)
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
	slices.SortFunc(out, func(a admissionregistrationv1.RuleWithOperations, b admissionregistrationv1.RuleWithOperations) int {
		if x, match := less(a.APIGroups, b.APIGroups); match {
			return x
		}
		if x, match := less(a.APIVersions, b.APIVersions); match {
			return x
		}
		if x, match := less(a.Resources, b.Resources); match {
			return x
		}
		if x := strings.Compare(string(*a.Scope), string(*b.Scope)); x != 0 {
			return x
		}
		return 0
	})
	return out
}

func (wh *webhook) set(
	group string,
	version string,
	resource string,
	subresource string,
	scope admissionregistrationv1.ScopeType,
	operations ...admissionregistrationv1.OperationType,
) {
	for _, operation := range operations {
		wh.rules.Insert(ruleEntry{
			group:       group,
			version:     version,
			resource:    resource,
			subresource: subresource,
			scope:       scope,
			operation:   operation,
		})
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
		name = name + "-finegrained-" + wh.key("-")
		path = path + config.FineGrainedWebhookPath + "/" + wh.key("/")
	}
	return name, path
}
