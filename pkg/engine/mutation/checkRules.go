package mutation

import (
	"github.com/minio/minio/pkg/wildcard"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// kind is the type of object being manipulated
// Checks requests kind, name and labels to fit the policy
func IsRuleApplicableToResource(resourceRaw []byte, description types.ResourceDescription) (bool, error) {
	kind := ParseKindFromObject(resourceRaw)
	if description.Kind != kind {
		return false, nil
	}

	if resourceRaw != nil {
		meta := ParseMetadataFromObject(resourceRaw)
		name := ParseNameFromObject(resourceRaw)

		if description.Name != nil {

			if !wildcard.Match(*description.Name, name) {
				return false, nil
			}
		}

		if description.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(description.Selector)

			if err != nil {
				return false, err
			}

			labelMap := ParseLabelsFromMetadata(meta)

			if !selector.Matches(labelMap) {
				return false, nil
			}

		}
	}
	return true, nil
}
