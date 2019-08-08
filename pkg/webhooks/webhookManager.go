package webhooks

import (
	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
)

func (ws *WebhookServer) registerWebhookConfigurations(policy v1alpha1.Policy) error {

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation != nil && !ws.webhookRegistrationClient.MutationRegistered.IsSet() {
			if err := ws.webhookRegistrationClient.RegisterMutatingWebhook(); err != nil {
				return err
			}

			ws.webhookRegistrationClient.MutationRegistered.Set()
			glog.Infof("Mutating webhook registered")
		}

		if rule.Validation != nil && !ws.webhookRegistrationClient.ValidationRegistered.IsSet() {
			if err := ws.webhookRegistrationClient.RegisterValidatingWebhook(); err != nil {
				return err
			}

			ws.webhookRegistrationClient.ValidationRegistered.Set()
			glog.Infof("Validating webhook registered")
		}
	}
	return nil
}
