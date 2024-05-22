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
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	rules             map[groupVersionScope]sets.Set[string]
	matchConditions   []admissionregistrationv1.MatchCondition
}

// groupVersionScope contains the GV and scopeType of a resource
type groupVersionScope struct {
	schema.GroupVersion
	scopeType admissionregistrationv1.ScopeType
}

// String puts / between group/version and scope
func (gvs groupVersionScope) String() string {
	return gvs.GroupVersion.String() + "/" + string(gvs.scopeType)
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             map[groupVersionScope]sets.Set[string]{},
		matchConditions:   matchConditions,
	}
}

func findKeyContainingSubstring(m map[string][]admissionregistrationv1.OperationType, substring string, defaultOpn []admissionregistrationv1.OperationType) []admissionregistrationv1.OperationType {
	for key, value := range m {
		if key == "Pod/exec" || strings.Contains(strings.ToLower(key), strings.ToLower(substring)) || strings.Contains(strings.ToLower(substring), strings.ToLower(key)) {
			return value
		}
	}
	return defaultOpn
}

func newWebhookPerPolicy(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition, policy kyvernov1.PolicyInterface) *webhook {
	webhook := newWebhook(timeout, failurePolicy, matchConditions)
	webhook.policyMeta = objectmeta.ObjectName{
		Namespace: policy.GetNamespace(),
		Name:      policy.GetName(),
	}
	if policy.GetSpec().CustomWebhookConfiguration() {
		webhook.matchConditions = policy.GetSpec().GetMatchConditions()
	}
	return webhook
}

func (wh *webhook) buildRulesWithOperations(final map[string][]admissionregistrationv1.OperationType, defaultOpn []admissionregistrationv1.OperationType) []admissionregistrationv1.RuleWithOperations {
	rules := make([]admissionregistrationv1.RuleWithOperations, 0, len(wh.rules))

	for gv, resources := range wh.rules {
		firstResource := sets.List(resources)[0]
		// if we have pods, we add pods/ephemeralcontainers by default
		if (gv.Group == "" || gv.Group == "*") && (gv.Version == "v1" || gv.Version == "*") && (resources.Has("pods") || resources.Has("*")) {
			resources.Insert("pods/ephemeralcontainers")
		}

		operations := findKeyContainingSubstring(final, firstResource, defaultOpn)
		if len(operations) == 0 {
			continue
		}

		slices.SortFunc(operations, func(a, b admissionregistrationv1.OperationType) int {
			return cmp.Compare(a, b)
		})

		rules = append(rules, admissionregistrationv1.RuleWithOperations{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{gv.Group},
				APIVersions: []string{gv.Version},
				Resources:   sets.List(resources),
				Scope:       ptr.To(gv.scopeType),
			},
			Operations: operations,
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
		if x := strings.Compare(string(*a.Scope), string(*b.Scope)); x != 0 {
			return x
		}
		return 0
	})
	return rules
}

func scanResourceFilterForResources(resFilter kyvernov1.ResourceFilters) []string {
	var resources []string
	for _, rf := range resFilter {
		if rf.ResourceDescription.Kinds != nil {
			resources = append(resources, rf.ResourceDescription.Kinds...)
		}
	}
	return resources
}

func scanResourceFilter(resFilter kyvernov1.ResourceFilters, operationStatusMap map[string]bool) (bool, map[string]bool) {
	opFound := false
	for _, rf := range resFilter {
		if rf.ResourceDescription.Operations != nil {
			for _, o := range rf.ResourceDescription.Operations {
				opFound = true
				operationStatusMap[string(o)] = true
			}
		}
	}
	return opFound, operationStatusMap
}

func scanResourceFilterForExclude(resFilter kyvernov1.ResourceFilters, operationStatusMap map[string]bool) (bool, map[string]bool) {
	opFound := false
	for _, rf := range resFilter {
		if rf.ResourceDescription.Operations != nil {
			for _, o := range rf.ResourceDescription.Operations {
				opFound = true
				operationStatusMap[string(o)] = false
			}
		}
	}
	return opFound, operationStatusMap
}

func (wh *webhook) set(gvrs GroupVersionResourceScope) {
	gvs := groupVersionScope{
		GroupVersion: gvrs.GroupVersion(),
		scopeType:    gvrs.Scope,
	}

	// check if the resource contains wildcard and is already added as all scope
	// in that case, we do not need to add it again as namespaced scope
	if (gvrs.Resource == "*" || gvrs.Group == "*") && gvs.scopeType == admissionregistrationv1.NamespacedScope {
		allScopeResource := groupVersionScope{
			GroupVersion: gvs.GroupVersion,
			scopeType:    admissionregistrationv1.AllScopes,
		}
		resources := wh.rules[allScopeResource]
		if resources != nil {
			// explicitly do nothing as the resource is already added as all scope
			return
		}
	}

	// check if the resource is already added
	resources := wh.rules[gvs]
	if resources == nil {
		wh.rules[gvs] = sets.New(gvrs.Resource)
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

func computeOperationsForValidatingWebhookConf(r kyvernov1.Rule, operationStatusMap map[string]bool) map[string]bool {
	var opFound bool
	opFoundCount := 0
	if len(r.MatchResources.Any) != 0 {
		opFound, operationStatusMap = scanResourceFilter(r.MatchResources.Any, operationStatusMap)
		opFoundCount = opFoundCountIncrement(opFound, opFoundCount)
	}
	if len(r.MatchResources.All) != 0 {
		opFound, operationStatusMap = scanResourceFilter(r.MatchResources.All, operationStatusMap)
		opFoundCount = opFoundCountIncrement(opFound, opFoundCount)
	}
	if r.MatchResources.ResourceDescription.Operations != nil {
		for _, o := range r.MatchResources.ResourceDescription.Operations {
			opFound = true
			operationStatusMap[string(o)] = true
			opFoundCount = opFoundCountIncrement(opFound, opFoundCount)
		}
	}
	if !opFound {
		operationStatusMap[webhookCreate] = true
		operationStatusMap[webhookUpdate] = true
		operationStatusMap[webhookConnect] = true
		operationStatusMap[webhookDelete] = true
	}
	if r.ExcludeResources.ResourceDescription.Operations != nil {
		for _, o := range r.ExcludeResources.ResourceDescription.Operations {
			operationStatusMap[string(o)] = false
		}
	}
	if len(r.ExcludeResources.Any) != 0 {
		_, operationStatusMap = scanResourceFilterForExclude(r.ExcludeResources.Any, operationStatusMap)
	}
	if len(r.ExcludeResources.All) != 0 {
		_, operationStatusMap = scanResourceFilterForExclude(r.ExcludeResources.All, operationStatusMap)
	}
	return operationStatusMap
}

func opFoundCountIncrement(opFound bool, opFoundCount int) int {
	if opFound {
		opFoundCount++
	}
	return opFoundCount
}

func computeOperationsForMutatingWebhookConf(r kyvernov1.Rule, operationStatusMap map[string]bool) map[string]bool {
	if r.HasMutate() || r.HasVerifyImages() {
		var opFound bool
		opFoundCount := 0
		if len(r.MatchResources.Any) != 0 {
			opFound, operationStatusMap = scanResourceFilter(r.MatchResources.Any, operationStatusMap)
			opFoundCount = opFoundCountIncrement(opFound, opFoundCount)
		}
		if len(r.MatchResources.All) != 0 {
			opFound, operationStatusMap = scanResourceFilter(r.MatchResources.All, operationStatusMap)
			opFoundCount = opFoundCountIncrement(opFound, opFoundCount)
		}
		if r.MatchResources.ResourceDescription.Operations != nil {
			for _, o := range r.MatchResources.ResourceDescription.Operations {
				opFound = true
				operationStatusMap[string(o)] = true
				opFoundCount = opFoundCountIncrement(opFound, opFoundCount)
			}
		}
		if opFoundCount == 0 {
			operationStatusMap[webhookCreate] = true
			operationStatusMap[webhookUpdate] = true
		}
		if r.ExcludeResources.ResourceDescription.Operations != nil {
			for _, o := range r.ExcludeResources.ResourceDescription.Operations {
				operationStatusMap[string(o)] = false
			}
		}
		if len(r.ExcludeResources.Any) != 0 {
			_, operationStatusMap = scanResourceFilterForExclude(r.ExcludeResources.Any, operationStatusMap)
		}
		if len(r.ExcludeResources.All) != 0 {
			_, operationStatusMap = scanResourceFilterForExclude(r.ExcludeResources.All, operationStatusMap)
		}
	}
	return operationStatusMap
}

func mergeOperations(operationStatusMap map[string]bool, currentOps []admissionregistrationv1.OperationType) []admissionregistrationv1.OperationType {
	operationReq := make([]admissionregistrationv1.OperationType, 0, 4)
	for k, v := range operationStatusMap {
		if v {
			var oper admissionregistrationv1.OperationType = admissionregistrationv1.OperationType(k)
			operationReq = append(operationReq, oper)
		}
	}
	result := sets.New(currentOps...).Insert(operationReq...)
	return result.UnsortedList()
}

func getOperationStatusMap() map[string]bool {
	operationStatusMap := make(map[string]bool)
	operationStatusMap[webhookCreate] = false
	operationStatusMap[webhookUpdate] = false
	operationStatusMap[webhookDelete] = false
	operationStatusMap[webhookConnect] = false
	return operationStatusMap
}

func appendResource(r string, mapResourceToOpn map[string]map[string]bool, opnStatusMap map[string]bool, mapResourceToOpnType map[string][]admissionregistrationv1.OperationType) (map[string]map[string]bool, map[string][]admissionregistrationv1.OperationType) {
	if _, exists := mapResourceToOpn[r]; exists {
		opnStatMap1 := opnStatusMap
		opnStatMap2 := mapResourceToOpn[r]
		for opn := range opnStatusMap {
			if opnStatMap1[opn] || opnStatMap2[opn] {
				opnStatusMap[opn] = true
			}
		}
		mapResourceToOpn[r] = opnStatusMap
		mapResourceToOpnType[r] = mergeOperations(opnStatusMap, mapResourceToOpnType[r])
	} else {
		if mapResourceToOpn == nil {
			mapResourceToOpn = make(map[string]map[string]bool)
		}
		mapResourceToOpn[r] = opnStatusMap
		if mapResourceToOpnType == nil {
			mapResourceToOpnType = make(map[string][]admissionregistrationv1.OperationType)
		}
		mapResourceToOpnType[r] = mergeOperations(opnStatusMap, mapResourceToOpnType[r])
	}
	return mapResourceToOpn, mapResourceToOpnType
}

func computeResourcesOfRule(r kyvernov1.Rule) []string {
	var resources []string
	if len(r.MatchResources.Any) != 0 {
		resources = scanResourceFilterForResources(r.MatchResources.Any)
	}
	if len(r.MatchResources.All) != 0 {
		resources = scanResourceFilterForResources(r.MatchResources.Any)
	}
	if len(r.ExcludeResources.Any) != 0 {
		resources = scanResourceFilterForResources(r.MatchResources.Any)
	}
	if len(r.ExcludeResources.All) != 0 {
		resources = scanResourceFilterForResources(r.MatchResources.Any)
	}
	if r.MatchResources.ResourceDescription.Kinds != nil {
		resources = append(resources, r.MatchResources.ResourceDescription.Kinds...)
	}
	if r.ExcludeResources.ResourceDescription.Kinds != nil {
		resources = append(resources, r.ExcludeResources.ResourceDescription.Kinds...)
	}
	return resources
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
