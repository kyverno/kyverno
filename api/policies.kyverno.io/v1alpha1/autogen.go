package v1alpha1

type PodControllersGenerationConfiguration struct {
	// TODO: shall we use GVK/GVR instead of string ?
	Controllers []string `json:"controllers,omitempty"`
}
