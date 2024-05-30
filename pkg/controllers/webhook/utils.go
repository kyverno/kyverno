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

func (wh *webhook) buildRulesWithOperations(final map[string][]admissionregistrationv1.OperationType, defaultOpn []admissionregistrationv1.OperationType) []admissionregistrationv1.RuleWithOperations {
	var rules []admissionregistrationv1.RuleWithOperations
	for gv, resources := range wh.rules {
		firstResource := sets.List(resources)[0]
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
			Operations: findKeyContainingSubstring(final, firstResource, defaultOpn),
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

func findKeyContainingSubstring(m map[string][]admissionregistrationv1.OperationType, substring string, defaultOpn []admissionregistrationv1.OperationType) []admissionregistrationv1.OperationType {
	for key, value := range m {
		if strings.Contains(strings.ToLower(key), strings.ToLower(substring)) || strings.Contains(strings.ToLower(substring), strings.ToLower(key)) {
			return value
		}
	}
	return defaultOpn
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

func getMinimumOperations(operationStatusMap map[string]bool) []admissionregistrationv1.OperationType {
	operationReq := make([]admissionregistrationv1.OperationType, 0, 4)
	for k, v := range operationStatusMap {
		if v {
			var oper admissionregistrationv1.OperationType = admissionregistrationv1.OperationType(k)
			operationReq = append(operationReq, oper)
		}
	}
	return operationReq
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
		mapResourceToOpnType[r] = getMinimumOperations(opnStatusMap)
	} else {
		if mapResourceToOpn == nil {
			mapResourceToOpn = make(map[string]map[string]bool)
		}
		mapResourceToOpn[r] = opnStatusMap
		if mapResourceToOpnType == nil {
			mapResourceToOpnType = make(map[string][]admissionregistrationv1.OperationType)
		}
		mapResourceToOpnType[r] = getMinimumOperations(opnStatusMap)
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
