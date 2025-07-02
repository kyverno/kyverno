package webhook

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"golang.org/x/exp/maps"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func extractGenericPolicy(policy engineapi.GenericPolicy) policiesv1alpha1.GenericPolicy {
	if vpol := policy.AsValidatingPolicy(); vpol != nil {
		return vpol
	}
	if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
		return ivpol
	}
	if gpol := policy.AsGeneratingPolicy(); gpol != nil {
		return gpol
	}
	if mpol := policy.AsMutatingPolicy(); mpol != nil {
		return mpol
	}
	return nil
}

func collectResourceDescriptions(rule kyvernov1.Rule, defaultOps ...kyvernov1.AdmissionOperation) webhookConfig {
	out := map[string]sets.Set[kyvernov1.AdmissionOperation]{}
	for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
		if out[kind] == nil {
			out[kind] = sets.New[kyvernov1.AdmissionOperation]()
		}
		ops := rule.MatchResources.ResourceDescription.Operations
		if len(ops) == 0 {
			ops = defaultOps
		}
		out[kind].Insert(ops...)
	}
	for _, value := range rule.MatchResources.All {
		for _, kind := range value.Kinds {
			if out[kind] == nil {
				out[kind] = sets.New[kyvernov1.AdmissionOperation]()
			}
			ops := value.Operations
			if len(ops) == 0 {
				ops = defaultOps
			}
			out[kind].Insert(ops...)
		}
	}
	for _, value := range rule.MatchResources.Any {
		for _, kind := range value.Kinds {
			if out[kind] == nil {
				out[kind] = sets.New[kyvernov1.AdmissionOperation]()
			}
			ops := value.Operations
			if len(ops) == 0 {
				ops = defaultOps
			}
			out[kind].Insert(ops...)
		}
	}
	// we consider only `exclude.any` elements and only if `kinds` is empty or if there's a corresponding kind in the match statement
	// nothing else than `kinds` and `operations` must be set
	if rule.ExcludeResources != nil {
		for _, value := range rule.ExcludeResources.Any {
			if !value.UserInfo.IsEmpty() {
				continue
			}
			if value.Name != "" ||
				len(value.Names) != 0 ||
				len(value.Namespaces) != 0 ||
				len(value.Annotations) != 0 ||
				value.Selector != nil ||
				value.NamespaceSelector != nil {
				continue
			}
			kinds := value.Kinds
			if len(kinds) == 0 {
				kinds = maps.Keys(out)
			}
			ops := value.Operations
			if len(ops) == 0 {
				// if only kind was specified, clear all operations
				ops = allOperations
			}
			for _, kind := range kinds {
				if out[kind] != nil {
					out[kind] = out[kind].Delete(ops...)
				}
			}
		}
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

const (
	ValidatingPolicyType      = "ValidatingPolicy"
	ImageValidatingPolicyType = "ImageValidatingPolicy"
	MutatingPolicyType        = "MutatingPolicy"
	GeneratingPolicyType      = "GeneratingPolicy"
)
