package v1

const (
	// PodControllersAnnotation defines the annotation key for Pod-Controllers
	PodControllersAnnotation = "pod-policies.kyverno.io/autogen-controllers"
	// ManagedByLabel defines the label key for managed-by label
	ManagedByLabel = "app.kubernetes.io/managed-by"
	// KyvernoAppValue defines the kyverno application value
	KyvernoAppValue = "kyverno"
)
