package constants

const (
	// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
	WebhookServiceNamespace = "kube-system"
	WebhookServiceName = "kube-policy-svc"

	WebhookConfigName = "nirmata-kube-policy-webhook-cfg"
	MutationWebhookName = "webhook.nirmata.kube-policy"
)

var (
	WebhookServicePath = "/mutate"
	WebhookConfigLabels = map[string]string {
		"app": "kube-policy",
	}
)