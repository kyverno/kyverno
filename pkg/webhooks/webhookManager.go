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

		if rule.Validation != nil && !ws.webhookRegistrationClient.ValidationRegistered.IsSet() {
			if err := ws.webhookRegistrationClient.RegisterValidatingWebhook(); err != nil {
				return err
			}
			glog.Infof("Validating webhook registered")
		}
	}
	return nil
}

func (ws *WebhookServer) deregisterWebhookConfigurations(policy v1alpha1.Policy) error {
	pt := none
	glog.V(3).Infof("Retreiving policy type for %s\n", policy.Name)

	for _, rule := range policy.Spec.Rules {
		if rule.Validation != nil {
			pt = pt | validate
		}

		if rule.Mutation != nil {
			pt = pt | mutate
		}
	}

	glog.V(3).Infof("Scanning policy type==%v\n", pt)

	existPolicyType := ws.isPolicyTypeExist(pt, policy.Name)
	glog.V(3).Infof("Found existing policy type==%v\n", existPolicyType)

	switch existPolicyType {
	case none:
		ws.webhookRegistrationClient.deregister()
		glog.Infoln("All webhook deregistered")
	case mutate:
		if pt != mutate {
			ws.webhookRegistrationClient.deregisterValidatingWebhook()
			glog.Infoln("Validating webhook deregistered")
		}
	case validate:
		if pt != validate {
			ws.webhookRegistrationClient.deregisterMutatingWebhook()
			glog.Infoln("Mutating webhook deregistered")
		}
	case all:
		return nil
	}

	return nil
}

func (ws *WebhookServer) isPolicyTypeExist(pt policyType, policyName string) policyType {
	ptype := none

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		glog.Errorf("Failed to get policy list")
	}

	for _, p := range policies {
		if p.Name == policyName {
			glog.Infof("Skipping policy type check on %s\n", policyName)
			continue
		}

		for _, rule := range p.Spec.Rules {
			if rule.Mutation != nil {
				ptype = ptype | mutate
			}

			if rule.Validation != nil {
				ptype = ptype | validate
			}
		}
	}

	return ptype
}
