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
	policyMeta objectmeta.ObjectName

	maxWebhookTimeout int32
	failurePolicy     admissionregistrationv1.FailurePolicyType
	rules             sets.Set[GroupVersionResourceScopeOperation]
	matchConditions   []admissionregistrationv1.MatchCondition
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             sets.New[GroupVersionResourceScopeOperation](),
		matchConditions:   matchConditions,
	}
}

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

func (wh *webhook) buildRules() []admissionregistrationv1.RuleWithOperations {
	// Group By GroupVersionResourceScope
	gvrsGroupedRules := make(map[string]*admissionregistrationv1.RuleWithOperations)
	for gvrso := range wh.rules {
		key := gvrso.GroupVersion().String() + "/" + gvrso.Resource + "/" + string(gvrso.Scope)

		if rule, exists := gvrsGroupedRules[key]; exists {
			rule.Operations = append(rule.Operations, gvrso.Operation)
		} else {
			resources := []string{gvrso.Resource}
			// if we have pods, we add pods/ephemeralcontainers by default
			if (gvrso.Group == "" || gvrso.Group == "*") && (gvrso.Version == "v1" || gvrso.Version == "*") && (gvrso.Resource == "pods" || gvrso.Resource == "*") {
				resources = append(resources, "pods/ephemeralcontainers")
			}

			gvrsGroupedRules[key] = &admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{gvrso.Operation},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{gvrso.Group},
					APIVersions: []string{gvrso.Version},
					Resources:   resources,
					Scope:       ptr.To(gvrso.Scope),
				},
			}
		}
	}

	// Group By GroupVersionScopeOperations
	gvsoGroupedRules := make(map[string]*admissionregistrationv1.RuleWithOperations)
	for _, rule := range gvrsGroupedRules {
		slices.SortFunc(rule.Operations, func(a, b admissionregistrationv1.OperationType) int {
			return cmp.Compare(a, b)
		})
		operations := make([]string, len(rule.Operations))
		for i, op := range rule.Operations {
			operations[i] = string(op)
		}
		key := rule.APIGroups[0] + "/" + rule.APIVersions[0] + "/" + string(*rule.Scope) + "/" + strings.Join(operations, ",")

		if groupedRule, exists := gvsoGroupedRules[key]; exists {
			groupedRule.Resources = append(groupedRule.Resources, rule.Resources...)
		} else {
			gvsoGroupedRules[key] = rule
		}
	}

	result := make([]admissionregistrationv1.RuleWithOperations, 0, len(gvrsGroupedRules))
	for _, rule := range gvsoGroupedRules {
		result = append(result, *rule)
	}

	for _, rule := range result {
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
	slices.SortFunc(result, func(a admissionregistrationv1.RuleWithOperations, b admissionregistrationv1.RuleWithOperations) int {
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

	return result
}

type RuleKey struct {
	Kind string
	Op   admissionregistrationv1.OperationType
}

// ExtractKindOpFromRule extracts kinds and operations from the rule
func ExtractKindOpFromRule(
	r *kyvernov1.Rule,
	kindOpSet sets.Set[RuleKey],
	defaultOps ...admissionregistrationv1.OperationType,
) {
	InsertKindOpusingResDescription(&r.MatchResources.ResourceDescription, kindOpSet, defaultOps...)
	for _, resFilter := range r.MatchResources.Any {
		InsertKindOpusingResDescription(&resFilter.ResourceDescription, kindOpSet, defaultOps...)
	}
	for _, resFilter := range r.MatchResources.All {
		InsertKindOpusingResDescription(&resFilter.ResourceDescription, kindOpSet, defaultOps...)
	}

	if r.ExcludeResources != nil {
		DeleteKindOpUsingResDescription(&r.ExcludeResources.ResourceDescription, kindOpSet)
		for _, resFilter := range r.ExcludeResources.Any {
			DeleteKindOpUsingResDescription(&resFilter.ResourceDescription, kindOpSet)
		}
		for _, resFilter := range r.ExcludeResources.All {
			DeleteKindOpUsingResDescription(&resFilter.ResourceDescription, kindOpSet)
		}
	}
}

// InsertKindOpusingResDescription adds kinds and operations from the resource description
func InsertKindOpusingResDescription(
	d *kyvernov1.ResourceDescription,
	kindOpSet sets.Set[RuleKey],
	defaultOps ...admissionregistrationv1.OperationType,
) {
	for _, kind := range d.Kinds {
		if len(d.Operations) > 0 {
			for _, op := range d.Operations {
				kindOpSet.Insert(RuleKey{Kind: kind, Op: admissionregistrationv1.OperationType(op)})
			}
		} else {
			InsertKindWithDefaultOperations([]string{kind}, kindOpSet, defaultOps...)
		}
	}
}

func InsertKindWithDefaultOperations(
	kinds []string,
	kindOpSet sets.Set[RuleKey],
	defaultOps ...admissionregistrationv1.OperationType,
) {
	for _, kind := range kinds {
		for _, op := range defaultOps {
			kindOpSet.Insert(RuleKey{Kind: kind, Op: op})
		}
	}
}

// DeleteKindOpUsingResDescription removes kinds and operations using the resource description
func DeleteKindOpUsingResDescription(
	d *kyvernov1.ResourceDescription,
	kindOpSet sets.Set[RuleKey],
) {
	if len(d.Kinds) == 0 {
		for gvko := range kindOpSet {
			for _, op := range d.Operations {
				if gvko.Op == admissionregistrationv1.OperationType(op) {
					kindOpSet.Delete(gvko)
				}
			}
		}
	}
	for _, kind := range d.Kinds {
		for _, op := range d.Operations {
			kindOpSet.Delete(RuleKey{Kind: kind, Op: admissionregistrationv1.OperationType(op)})
		}
	}
}

func (wh *webhook) set(gvrso GroupVersionResourceScopeOperation) {
	// check if the resource contains wildcard and is already added as all scope
	// in that case, we do not need to add it again as namespaced scope
	if (gvrso.Resource == "*" || gvrso.Group == "*") && gvrso.Scope == admissionregistrationv1.NamespacedScope {
		allScopeResource := GroupVersionResourceScopeOperation{
			GroupVersionResource: gvrso.GroupVersionResource,
			Scope:                admissionregistrationv1.AllScopes,
			Operation:            gvrso.Operation,
		}
		if wh.rules.Has(allScopeResource) {
			// explicitly do nothing as the resource is already added as all scope
			return
		}
	}

	wh.rules.Insert(gvrso)
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
