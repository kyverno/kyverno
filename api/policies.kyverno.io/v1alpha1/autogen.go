package v1alpha1

type PodControllersGenerationConfiguration struct {
	Controllers []string `json:"controllers,omitempty"`
}

type Target struct {
	Group    string `json:"group,omitempty"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
	Kind     string `json:"kind"`
}

type ValidatingPolicyAutogenStatus struct {
	Configs map[string]ValidatingPolicyAutogen `json:"configs,omitempty"`
}

type ImageValidatingPolicyAutogenStatus struct {
	Configs map[string]ImageValidatingPolicyAutogen `json:"configs,omitempty"`
}

type MutatingPolicyAutogenStatus struct {
	Configs map[string]MutatingPolicyAutogen `json:"configs,omitempty"`
}

type ValidatingPolicyAutogen struct {
	Targets []Target              `json:"targets"`
	Spec    *ValidatingPolicySpec `json:"spec"`
}

type ImageValidatingPolicyAutogen struct {
	Targets []Target                   `json:"targets"`
	Spec    *ImageValidatingPolicySpec `json:"spec"`
}

type MutatingPolicyAutogen struct {
	Targets []Target            `json:"targets"`
	Spec    *MutatingPolicySpec `json:"spec"`
}
