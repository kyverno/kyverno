package v1

const (
	// PodControllersAnnotation defines the annotation key for Pod-Controllers
	PodControllersAnnotation = "pod-policies.kyverno.io/autogen-controllers"
	// LabelAppManagedBy defines the label key for managed-by label
	LabelAppManagedBy = "app.kubernetes.io/managed-by"
	// ValueKyvernoApp defines the kyverno application value
	ValueKyvernoApp = "kyverno"
)
