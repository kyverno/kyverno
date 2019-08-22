package webhooks

import (
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
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

func (ws *WebhookServer) manageWebhookConfigurations(policy kyverno.Policy, op v1beta1.Operation) {
	switch op {
	case v1beta1.Create:
		ws.registerWebhookConfigurations(policy)
	case v1beta1.Delete:
		ws.deregisterWebhookConfigurations(policy)
	}
}

func (ws *WebhookServer) registerWebhookConfigurations(policy kyverno.Policy) error {
	if !HasMutateOrValidate(policy) {
		return nil
	}

	if !ws.webhookRegistrationClient.MutationRegistered.IsSet() {
		if err := ws.webhookRegistrationClient.RegisterMutatingWebhook(); err != nil {
			return err
		}
		glog.Infof("Mutating webhook registered")
	}

	return nil
}

func (ws *WebhookServer) deregisterWebhookConfigurations(policy kyverno.Policy) error {
	policies, _ := ws.pLister.List(labels.NewSelector())

	// deregister webhook if no mutate/validate policy found in cluster
	if !HasMutateOrValidatePolicies(policies) {
		ws.webhookRegistrationClient.DeregisterMutatingWebhook()
		glog.Infoln("Mutating webhook deregistered")
	}

	return nil
}

func HasMutateOrValidatePolicies(policies []*kyverno.Policy) bool {
	for _, policy := range policies {
		if HasMutateOrValidate(*policy) {
			return true
		}
	}
	return false
}

func HasMutateOrValidate(policy kyverno.Policy) bool {
	for _, rule := range policy.Spec.Rules {
		if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) || !reflect.DeepEqual(rule.Validation, kyverno.Validation{}) {
			glog.Infoln(rule.Name)
			return true
		}
	}
	return false
}
