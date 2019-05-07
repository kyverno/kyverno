package webhooks

import (
	"github.com/minio/minio/pkg/wildcard"
	kubeclient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func kindIsSupported(kind string) bool {
	for _, k := range kubeclient.GetSupportedKinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// Checks for admission if kind is supported
func AdmissionIsRequired(request *v1beta1.AdmissionRequest) bool {
	// Here you can make additional hardcoded checks
	return kindIsSupported(request.Kind.Kind)
}

// Checks requests kind, name and labels to fit the policy
func IsRuleApplicableToRequest(policyResource types.PolicyResource, request *v1beta1.AdmissionRequest) (bool, error) {
	return IsRuleApplicableToResource(request.Kind.Kind, request.Object.Raw, policyResource)
}

// kind is the type of object being manipulated
// Checks requests kind, name and labels to fit the policy
func IsRuleApplicableToResource(kind string, resourceRaw []byte, policyResource types.PolicyResource) (bool, error) {
	if policyResource.Kind != kind {
		return false, nil
	}

	if resourceRaw != nil {
		meta := parseMetadataFromObject(resourceRaw)
		name := parseNameFromObject(resourceRaw)

		if policyResource.Name != nil {

			if !wildcard.Match(*policyResource.Name, name) {
				return false, nil
			}
		}

		if policyResource.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policyResource.Selector)

			if err != nil {
				return false, err
			}

			labelMap := parseLabelsFromMetadata(meta)

			if !selector.Matches(labelMap) {
				return false, nil
			}

		}
	}
	return true, nil
}
