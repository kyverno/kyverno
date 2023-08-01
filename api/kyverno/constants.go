package kyverno

const (
	// Well known labels
	LabelAppComponent     = "app.kubernetes.io/component"
	LabelAppManagedBy     = "app.kubernetes.io/managed-by"
	LabelCacheEnabled     = "cache.kyverno.io/enabled"
	LabelCertManagedBy    = "cert.kyverno.io/managed-by"
	LabelCleanupTtl       = "cleanup.kyverno.io/ttl"
	LabelWebhookManagedBy = "webhook.kyverno.io/managed-by"
	// Well known annotations
	AnnotationPolicyCategory     = "policies.kyverno.io/category"
	AnnotationPolicyScored       = "policies.kyverno.io/scored"
	AnnotationPolicySeverity     = "policies.kyverno.io/severity"
	AnnotationAutogenControllers = "pod-policies.kyverno.io/autogen-controllers"
	// Well known values
	ValueKyvernoApp = "kyverno"
)
