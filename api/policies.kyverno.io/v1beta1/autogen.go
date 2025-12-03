package v1beta1

import "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"

type (
	PodControllersGenerationConfiguration = v1alpha1.PodControllersGenerationConfiguration
	Target                                = v1alpha1.Target
	ValidatingPolicyAutogenStatus         = v1alpha1.ValidatingPolicyAutogenStatus
	ValidatingPolicyAutogen               = v1alpha1.ValidatingPolicyAutogen
	ImageValidatingPolicyAutogenStatus    = v1alpha1.ImageValidatingPolicyAutogenStatus
	ImageValidatingPolicyAutogen          = v1alpha1.ImageValidatingPolicyAutogen
	MutatingPolicyAutogenStatus           = v1alpha1.MutatingPolicyAutogenStatus
	MutatingPolicyAutogen                 = v1alpha1.MutatingPolicyAutogen
)
