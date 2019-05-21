package config

const (
	// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
	KubePolicyNamespace = "kube-system"
	WebhookServiceName  = "kube-policy-svc"

	MutatingWebhookConfigurationName = "kube-policy-mutating-webhook-cfg"
	MutatingWebhookName              = "nirmata.kube-policy.mutating-webhook"

	ValidatingWebhookConfigurationName = "kube-policy-validating-webhook-cfg"
	ValidatingWebhookName              = "nirmata.kube-policy.validating-webhook"

	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Issue: https://github.com/kubernetes/kubernetes/pull/63972
	// When the issue is closed, we should use TypeMeta struct instead of this constants
	DeploymentKind           = "Deployment"
	DeploymentAPIVersion     = "extensions/v1beta1"
	KubePolicyDeploymentName = "kube-policy-deployment"
)

var (
	MutatingWebhookServicePath   = "/mutate"
	ValidatingWebhookServicePath = "/validate"
	KubePolicyAppLabels          = map[string]string{
		"app": "kube-policy",
	}

	SupportedKinds = []string{
		"ConfigMap",
		"CronJob",
		"DaemonSet",
		"Deployment",
		"Endpoints",
		"HorizontalPodAutoscaler",
		"Ingress",
		"Job",
		"LimitRange",
		"Namespace",
		"NetworkPolicy",
		"PersistentVolumeClaim",
		"PodDisruptionBudget",
		"PodTemplate",
		"ResourceQuota",
		"Secret",
		"Service",
		"StatefulSet",
	}
)
