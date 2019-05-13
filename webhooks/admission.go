package webhooks

import (
	kubeclient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	mutation "github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	"k8s.io/api/admission/v1beta1"
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
func IsRuleApplicableToRequest(policyResource types.ResourceDescription, request *v1beta1.AdmissionRequest) (bool, error) {
	return mutation.IsRuleApplicableToResource(request.Object.Raw, policyResource)
}
