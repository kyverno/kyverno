package kyverno

const (
	// Well known labels
	LabelAppManagedBy = "app.kubernetes.io/managed-by"
	LabelCleanupTtl   = "cleanup.kyverno.io/ttl"
	// Well known annotations
	AnnotationAutogenControllers = "pod-policies.kyverno.io/autogen-controllers"
	AnnotationPolicyCategory     = "policies.kyverno.io/category"
	AnnotationPolicySeverity     = "policies.kyverno.io/severity"
	AnnotationPolicyScored       = "policies.kyverno.io/scored"
	// Well known values
	ValueKyvernoApp = "kyverno"
)
