package policy

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) removeResourceWebhookConfiguration() error {
	removeWebhookConfig := func() error {
		var err error
		// check informer cache
		configName := pc.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
		config, err := pc.mWebhookConfigLister.Get(configName)
		if err != nil {
			glog.V(4).Infof("failed to list mutating webhook config: %v", err)
			return err
		}
		if config == nil {
			// as no resource is found
			return nil
		}
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

	// if polices only have generate rules, we dont need the webhook
	if !hasMutateOrValidatePolicies(policies) {
		glog.V(4).Info("no policies with mutating or validating webhook configurations, remove resource webhook configuration if one exists")
		return removeWebhookConfig()
	}

	return nil
}

func (pc *PolicyController) createResourceMutatingWebhookConfigurationIfRequired(policy kyverno.ClusterPolicy) error {
	// if the policy contains mutating & validation rules and it config does not exist we create one
	if policy.HasMutateOrValidate() {
		// check cache
		configName := pc.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
		config, err := pc.mWebhookConfigLister.Get(configName)
		if err != nil {
			glog.V(4).Infof("failed to list mutating webhook configuration: %v", err)
			return err
		}
		if config != nil {
			// mutating webhoook configuration already exists
			return nil
		}
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
