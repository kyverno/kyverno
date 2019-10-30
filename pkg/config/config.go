package config

import "flag"

const (
	// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
	KubePolicyNamespace = "kyverno"
	WebhookServiceName  = "kyverno-svc"

	MutatingWebhookConfigurationName      = "kyverno-resource-mutating-webhook-cfg"
	MutatingWebhookConfigurationDebugName = "kyverno-resource-mutating-webhook-cfg-debug"
	MutatingWebhookName                   = "nirmata.kyverno.resource.mutating-webhook"

	// ValidatingWebhookConfigurationName  = "kyverno-validating-webhook-cfg"
	// ValidatingWebhookConfigurationDebug = "kyverno-validating-webhook-cfg-debug"
	// ValidatingWebhookName               = "nirmata.kyverno.policy-validating-webhook"

	VerifyMutatingWebhookConfigurationName      = "kyverno-verify-mutating-webhook-cfg"
	VerifyMutatingWebhookConfigurationDebugName = "kyverno-verify-mutating-webhook-cfg-debug"
	VerifyMutatingWebhookName                   = "nirmata.kyverno.verify-mutating-webhook"

	PolicyValidatingWebhookConfigurationName      = "kyverno-policy-validating-webhook-cfg"
	PolicyValidatingWebhookConfigurationDebugName = "kyverno-policy-validating-webhook-cfg-debug"
	PolicyValidatingWebhookName                   = "nirmata.kyverno.policy-validating-webhook"

	PolicyMutatingWebhookConfigurationName      = "kyverno-policy-mutating-webhook-cfg"
	PolicyMutatingWebhookConfigurationDebugName = "kyverno-policy-mutating-webhook-cfg-debug"
	PolicyMutatingWebhookName                   = "nirmata.kyverno.policy-mutating-webhook"

	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Issue: https://github.com/kubernetes/kubernetes/pull/63972
	// When the issue is closed, we should use TypeMeta struct instead of this constants
	DeploymentKind           = "Deployment"
	DeploymentAPIVersion     = "extensions/v1beta1"
	KubePolicyDeploymentName = "kyverno"
)

var (
	MutatingWebhookServicePath         = "/mutate"
	ValidatingWebhookServicePath       = "/validate"
	PolicyValidatingWebhookServicePath = "/policyvalidate"
	PolicyMutatingWebhookServicePath   = "/policymutate"
	VerifyMutatingWebhookServicePath   = "/verifymutate"

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
