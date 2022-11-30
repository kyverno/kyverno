package metrics

import "go.opentelemetry.io/otel/attribute"

const (
	// keys
	RequestWebhookKey = attribute.Key("request_webhook")
)

var (
	// keyvalues
	WebhookMutating   = RequestWebhookKey.String("MutatingWebhookConfiguration")
	WebhookValidating = RequestWebhookKey.String("ValidatingWebhookConfiguration")
)
