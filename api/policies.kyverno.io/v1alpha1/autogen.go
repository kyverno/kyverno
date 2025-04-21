package v1alpha1

type PodControllersGenerationConfiguration struct {
	Controllers []string `json:"controllers,omitempty"`
}

type ImageValidatingPolicyAutogenStatus struct {
	Configs map[string]ImageValidatingPolicyAutogen `json:"configs,omitempty"`
}

type ImageValidatingPolicyAutogen struct {
	Spec *ImageValidatingPolicySpec `json:"spec"`
}

type ValidatingPolicyAutogenStatus struct {
	Configs map[string]ValidatingPolicyAutogen `json:"configs,omitempty"`
}

type ValidatingPolicyAutogen struct {
	Spec *ValidatingPolicySpec `json:"spec"`
}
