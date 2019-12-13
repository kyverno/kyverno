package policy

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
