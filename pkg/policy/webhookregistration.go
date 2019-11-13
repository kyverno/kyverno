package policy

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) removeResourceWebhookConfiguration() error {
	removeWebhookConfig := func() error {
		var err error
		err = pc.webhookRegistrationClient.RemoveResourceMutatingWebhookConfiguration()
		if err != nil {
			return err
		}
		glog.V(4).Info("removed resource webhook configuration")
		return nil
	}

	var err error
	// get all existing policies
	policies, err := pc.pLister.List(labels.NewSelector())
	if err != nil {
		glog.V(4).Infof("failed to list policies: %v", err)
		return err
	}

	if len(policies) == 0 {
		glog.V(4).Info("no policies loaded, removing resource webhook configuration if one exists")
		return removeWebhookConfig()
	}

	// if there are policies, check if they contain mutating or validating rule
	if !hasMutateOrValidatePolicies(policies) {
		glog.V(4).Info("no policies with mutating or validating webhook configurations, remove resource webhook configuration if one exists")
		return removeWebhookConfig()
	}

	return nil
}

func (pc *PolicyController) createResourceMutatingWebhookConfigurationIfRequired(policy kyverno.ClusterPolicy) error {
	// if the policy contains mutating & validation rules and it config does not exist we create one
	if policy.HasMutateOrValidate() {
		if err := pc.webhookRegistrationClient.CreateResourceMutatingWebhookConfiguration(); err != nil {
			return err
		}
	}
	return nil
}

func hasMutateOrValidatePolicies(policies []*kyverno.ClusterPolicy) bool {
	for _, policy := range policies {
		if (*policy).HasMutateOrValidate() {
			return true
		}
	}
	return false
}
