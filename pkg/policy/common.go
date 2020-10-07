package policy

import (
	"fmt"
	"strings"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func buildPolicyLabel(policyName string) (labels.Selector, error) {
	policyLabelmap := map[string]string{"policy": policyName}
	//NOt using a field selector, as the match function will have to cast the runtime.object
	// to get the field, while it can get labels directly, saves the cast effort
	ls := &metav1.LabelSelector{}
	if err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&policyLabelmap, ls, nil); err != nil {
		return nil, fmt.Errorf("failed to generate label sector of Policy name %s: %v", policyName, err)
	}
	policySelector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, fmt.Errorf("Policy %s has invalid label selector: %v", policyName, err)
	}
	return policySelector, nil
}

func transformResource(resource unstructured.Unstructured) []byte {
	data, err := resource.MarshalJSON()
	if err != nil {
		log.Log.Error(err, "failed to marshal resource")
		return nil
	}
	return data
}

// convertPoliciesToClusterPolicies - convert array of Policy to array of ClusterPolicy
func convertPoliciesToClusterPolicies(nsPolicies []*kyverno.Policy) []*kyverno.ClusterPolicy {
	var cpols []*kyverno.ClusterPolicy
	for _, pol := range nsPolicies {
		cpol := kyverno.ClusterPolicy(*pol)
		cpols = append(cpols, &cpol)
	}
	return cpols
}

// convertPolicyToClusterPolicy - convert Policy to ClusterPolicy
func convertPolicyToClusterPolicy(nsPolicies *kyverno.Policy) *kyverno.ClusterPolicy {
	cpol := kyverno.ClusterPolicy(*nsPolicies)
	return &cpol
}

func getIsNamespacedPolicy(key string) (string, string, bool) {
	namespace := ""
	index := strings.Index(key, "/")
	if index != -1 {
		namespace = key[:index]
		key = key[index+1:]
		return namespace, key, true
	}
	return namespace, key, false
}
