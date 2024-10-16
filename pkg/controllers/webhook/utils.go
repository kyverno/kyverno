package webhook

import (
	"cmp"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"golang.org/x/exp/maps"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//	func findKeyContainingSubstring(m map[string][]admissionregistrationv1.OperationType, substring string, defaultOpn []admissionregistrationv1.OperationType) []admissionregistrationv1.OperationType {
//		for key, value := range m {
//			if key == "Pod/exec" || strings.Contains(strings.ToLower(key), strings.ToLower(substring)) || strings.Contains(strings.ToLower(substring), strings.ToLower(key)) {
//				return value
//			}
//		}
//		return defaultOpn
//	}

func collectResourceDescriptions(rule kyvernov1.Rule) []kyvernov1.ResourceDescription {
	var out []kyvernov1.ResourceDescription //nolint:prealloc
	out = append(out, rule.MatchResources.ResourceDescription)
	for _, value := range rule.MatchResources.All {
		out = append(out, value.ResourceDescription)
	}
	for _, value := range rule.MatchResources.Any {
		out = append(out, value.ResourceDescription)
	}
	return out
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

func less[T cmp.Ordered](a []T, b []T) (int, bool) {
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
