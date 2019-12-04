package policy

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) removeResourceWebhookConfiguration() error {
	var err error
	// get all existing policies
	policies, err := pc.pLister.List(labels.NewSelector())
	if err != nil {
		glog.V(4).Infof("failed to list policies: %v", err)
		return err
	}

	if len(policies) == 0 {
		glog.V(4).Info("no policies loaded, removing resource webhook configuration if one exists")
		return pc.resourceWebhookWatcher.RemoveResourceWebhookConfiguration()
	}

	// if polices only have generate rules, we dont need the webhook
	if !hasMutateOrValidatePolicies(policies) {
		glog.V(4).Info("no policies with mutating or validating webhook configurations, remove resource webhook configuration if one exists")
		return pc.resourceWebhookWatcher.RemoveResourceWebhookConfiguration()
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
