package webhook

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	policiesv1v1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func extractGenericPolicy(policy engineapi.GenericPolicy) policiesv1v1beta1.GenericPolicy {
	if vpol := policy.AsValidatingPolicy(); vpol != nil {
		return vpol
	}
	if nvpol := policy.AsNamespacedValidatingPolicy(); nvpol != nil {
		return nvpol
	}
	if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
		return ivpol
	}
	if nivpol := policy.AsNamespacedImageValidatingPolicy(); nivpol != nil {
		return nivpol
	}
	if gpol := policy.AsGeneratingPolicy(); gpol != nil {
		return gpol
	}
	if ngpol := policy.AsNamespacedGeneratingPolicy(); ngpol != nil {
		return ngpol
	}
	if mpol := policy.AsMutatingPolicy(); mpol != nil {
		return mpol
	}
	if nmpol := policy.AsNamespacedMutatingPolicy(); nmpol != nil {
		return nmpol
	}
	return nil
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

func capTimeout(maxWebhookTimeout int32) int32 {
	if maxWebhookTimeout > 30 {
		return 30
	}
	return maxWebhookTimeout
}

func newClientConfig(server string, servicePort int32, caBundle []byte, path string) admissionregistrationv1.WebhookClientConfig {
	clientConfig := admissionregistrationv1.WebhookClientConfig{
		CABundle: caBundle,
	}
	if server == "" {
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Namespace: config.KyvernoNamespace(),
			Name:      config.KyvernoServiceName(),
			Path:      &path,
			Port:      &servicePort,
		}
	} else {
		url := fmt.Sprintf("https://%s%s", server, path)
		clientConfig.URL = &url
	}
	return clientConfig
}

func less[T cmp.Ordered](a []T, b []T) int {
	if x := cmp.Compare(len(a), len(b)); x != 0 {
		return x
	}
	for i := range a {
		if x := cmp.Compare(a[i], b[i]); x != 0 {
			return x
		}
	}
	return 0
}

func deDuplicatedRules(rules []admissionregistrationv1.RuleWithOperations) []admissionregistrationv1.RuleWithOperations {
	seen := make(map[string]struct{})
	uniqueRules := make([]admissionregistrationv1.RuleWithOperations, 0, len(rules))

	for _, rule := range rules {
		key := generateRuleKey(rule)
		if _, exist := seen[key]; !exist {
			seen[key] = struct{}{}
			uniqueRules = append(uniqueRules, rule)
		}
	}
	return uniqueRules
}

func scopeString(s *admissionregistrationv1.ScopeType) string {
	if s == nil {
		return string(admissionregistrationv1.AllScopes)
	}
	return string(*s)
}

func sortedRules(rules []admissionregistrationv1.RuleWithOperations) []admissionregistrationv1.RuleWithOperations {
	out := make([]admissionregistrationv1.RuleWithOperations, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rule)
	}

	for _, rule := range out {
		slices.Sort(rule.APIGroups)
		slices.Sort(rule.APIVersions)
		slices.Sort(rule.Resources)
		slices.Sort(rule.Operations)
	}
	slices.SortFunc(out, func(a admissionregistrationv1.RuleWithOperations, b admissionregistrationv1.RuleWithOperations) int {
		if x := less(a.APIGroups, b.APIGroups); x != 0 {
			return x
		}
		if x := less(a.APIVersions, b.APIVersions); x != 0 {
			return x
		}
		if x := less(a.Resources, b.Resources); x != 0 {
			return x
		}
		if x := less(a.Operations, b.Operations); x != 0 {
			return x
		}
		if x := strings.Compare(scopeString(a.Scope), scopeString(b.Scope)); x != 0 {
			return x
		}
		return 0
	})

	return out
}

func generateRuleKey(rule admissionregistrationv1.RuleWithOperations) string {
	sb := strings.Builder{}
	stringBuilderFn := func(input []string) {
		copiedInputs := make([]string, len(input))
		copy(copiedInputs, input)
		sort.Strings(copiedInputs)
		sb.WriteString(strings.Join(copiedInputs, ","))
		sb.WriteString("|")
	}

	stringBuilderFn(rule.APIGroups)
	stringBuilderFn(rule.APIVersions)
	stringBuilderFn(rule.Resources)

	opsCopy := make([]string, len(rule.Operations))
	for i, op := range rule.Operations {
		opsCopy[i] = string(op)
	}
	stringBuilderFn(opsCopy)

	sb.WriteString("s:")
	sb.WriteString(scopeString(rule.Scope))
	return sb.String()
}

const (
	ValidatingPolicyType                = "ValidatingPolicy"
	NamespacedValidatingPolicyType      = "NamespacedValidatingPolicy"
	ImageValidatingPolicyType           = "ImageValidatingPolicy"
	NamespacedImageValidatingPolicyType = "NamespacedImageValidatingPolicy"
	MutatingPolicyType                  = "MutatingPolicy"
	NamespacedMutatingPolicyType        = "NamespacedMutatingPolicy"
	GeneratingPolicyType                = "GeneratingPolicy"
	NamespacedGeneratingPolicyType      = "NamespacedGeneratingPolicy"
	DeletingPolicyType                  = "DeletingPolicy"
	NamespacedDeletingPolicyType        = "NamespacedDeletingPolicy"
)
