package config

import "flag"

const (
	// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
	KubePolicyNamespace = "kyverno"
	WebhookServiceName  = "kyverno-svc"

	MutatingWebhookConfigurationName = "kyverno-mutating-webhook-cfg"
	MutatingWebhookName              = "nirmata.kyverno.mutating-webhook"

	ValidatingWebhookConfigurationName = "kyverno-validating-webhook-cfg"
	ValidatingWebhookName              = "nirmata.kyverno.validating-webhook"

	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Issue: https://github.com/kubernetes/kubernetes/pull/63972
	// When the issue is closed, we should use TypeMeta struct instead of this constants
	DeploymentKind           = "Deployment"
	DeploymentAPIVersion     = "extensions/v1beta1"
	KubePolicyDeploymentName = "kyverno-deployment"
)

var (
	MutatingWebhookServicePath   = "/mutate"
	ValidatingWebhookServicePath = "/validate"
	KubePolicyAppLabels          = map[string]string{
		"app": "kyverno",
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

//LogDefaults sets default glog flags
func LogDefaultFlags() {
	flag.Set("logtostderr", "true")
	flag.Set("stderrthreshold", "WARNING")
	flag.Set("v", "2")
}
