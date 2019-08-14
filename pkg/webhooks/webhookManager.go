package webhooks

import (
	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type policyType int

const (
	none policyType = iota
	mutate
	validate
	all
)

func (ws *WebhookServer) manageWebhookConfigurations(policy v1alpha1.Policy, op v1beta1.Operation) {
	switch op {
	case v1beta1.Create:
		ws.registerWebhookConfigurations(policy)
	case v1beta1.Delete:
		ws.deregisterWebhookConfigurations(policy)
	}
}

func (ws *WebhookServer) registerWebhookConfigurations(policy v1alpha1.Policy) error {
	for _, rule := range policy.Spec.Rules {
		if rule.Mutation != nil && !ws.webhookRegistrationClient.MutationRegistered.IsSet() {
			if err := ws.webhookRegistrationClient.RegisterMutatingWebhook(); err != nil {
				return err
			}
			glog.Infof("Mutating webhook registered")
		}
	}
	return nil
}

func (ws *WebhookServer) deregisterWebhookConfigurations(policy v1alpha1.Policy) error {
	policies, _ := ws.policyLister.List(labels.NewSelector())

	// deregister webhook if no policy found in cluster
	if len(policies) == 1 {
		ws.webhookRegistrationClient.deregisterMutatingWebhook()
		glog.Infoln("Mutating webhook deregistered")
	}

	return nil
}
