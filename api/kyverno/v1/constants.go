package v1

const (
	// PodControllersAnnotation defines the annotation key for Pod-Controllers
	PodControllersAnnotation = "pod-policies.kyverno.io/autogen-controllers"
	// LabelAppManagedBy defines the label key for managed-by label
	LabelAppManagedBy        = "app.kubernetes.io/managed-by"
	AnnotationPolicyCategory = "policies.kyverno.io/category"
	AnnotationPolicySeverity = "policies.kyverno.io/severity"
	AnnotationPolicyScored   = "policies.kyverno.io/scored"
	// ValueKyvernoApp defines the kyverno application value
	ValueKyvernoApp = "kyverno"
)
