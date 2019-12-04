package resourcewebhookwatcher

import (
	"github.com/golang/glog"
)

func (rww *ResourceWebhookWatcher) RemoveResourceWebhookConfiguration() error {
	var err error
	// check informer cache
	configName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
	config, err := rww.mWebhookConfigLister.Get(configName)
	if err != nil {
		glog.V(4).Infof("failed to list mutating webhook config: %v", err)
		return err
	}
	if config == nil {
		// as no resource is found
		return nil
	}
	err = rww.webhookRegistrationClient.RemoveResourceMutatingWebhookConfiguration()
	if err != nil {
		return err
	}
	glog.V(3).Info("removed resource webhook configuration")
	return nil
}
