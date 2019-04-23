package config

const (
	// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
	KubePolicyDeploymentName = "kube-policy-deployment"
	KubePolicyNamespace      = "kube-system"
	WebhookServiceName       = "kube-policy-svc"
	WebhookConfigName        = "nirmata-kube-policy-webhook-cfg"
	MutationWebhookName      = "webhook.nirmata.kube-policy"

	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Pull request: https://github.com/kubernetes/kubernetes/pull/63972
	// When pull request is closed, we should use TypeMeta struct instead of this constants
	DeploymentKind       = "Deployment"
	DeploymentAPIVersion = "extensions/v1beta1"
)

var (
	WebhookServicePath  = "/mutate"
	WebhookConfigLabels = map[string]string{
		"app": "kube-policy",
	}
)
