package resourcewebhookwatcher

import (
	"time"

	"github.com/golang/glog"
	checker "github.com/nirmata/kyverno/pkg/checker"
	webhookconfig "github.com/nirmata/kyverno/pkg/webhookconfig"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	mconfiginformer "k8s.io/client-go/informers/admissionregistration/v1beta1"
	mconfiglister "k8s.io/client-go/listers/admissionregistration/v1beta1"
	cache "k8s.io/client-go/tools/cache"
)

type ResourceWebhookWatcher struct {
	lastReqTime *checker.LastReqTime
	// ch holds the requests to create resource mutatingwebhookconfiguration
	ch                   chan bool
	mwebhookconfigSynced cache.InformerSynced
	// list/get mutatingwebhookconfigurations
	mWebhookConfigLister      mconfiglister.MutatingWebhookConfigurationLister
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient
}

func NewResourceWebhookWatcher(
	lastReqTime *checker.LastReqTime,
	mconfigwebhookinformer mconfiginformer.MutatingWebhookConfigurationInformer,
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient,
) *ResourceWebhookWatcher {
	return &ResourceWebhookWatcher{
		lastReqTime:               lastReqTime,
		ch:                        make(chan bool),
		mwebhookconfigSynced:      mconfigwebhookinformer.Informer().HasSynced,
		mWebhookConfigLister:      mconfigwebhookinformer.Lister(),
		webhookRegistrationClient: webhookRegistrationClient,
	}
}

func (rww *ResourceWebhookWatcher) RegisterResourceWebhook() {
	rww.ch <- true
}

func (rww *ResourceWebhookWatcher) Run(stopCh <-chan struct{}) {
	glog.Info("Starting resource webhook watcher")
	defer glog.Info("Shutting down resource webhook watcher")

	// wait for cache to populate first time
	if !cache.WaitForCacheSync(stopCh, rww.mwebhookconfigSynced) {
		glog.Error("configuration: failed to sync webhook informer cache")
	}

	createWebhook := func() {
		if err := rww.createResourceMutatingWebhookConfigurationIfRequired(); err != nil {
			glog.Errorf("failed to create resource mutating webhook configuration: %v, re-queue creation request", err)
			rww.RegisterResourceWebhook()
		}
	}

	for {
		select {
		case <-rww.ch:
			timeDiff := time.Since(rww.lastReqTime.Time())
			if timeDiff < checker.DefaultDeadline {
				glog.V(3).Info("Verified webhook status, creating webhook configuration")
				go createWebhook()
			} else {
				glog.Info("Webhook is inactive, not creating resource webhook configuration")
			}

		case <-stopCh:
			glog.V(2).Infof("stopping resource webhook watcher")
			return
		}
	}
}

// CreateResourceMutatingWebhookConfigurationIfRequired creates a Mutatingwebhookconfiguration
// for all resource types if there's no mutatingwebhookcfg for existing policy
func (rww *ResourceWebhookWatcher) createResourceMutatingWebhookConfigurationIfRequired() error {
	// check cache
	configName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
	config, err := rww.mWebhookConfigLister.Get(configName)
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.V(4).Infof("failed to list mutating webhook configuration: %v", err)
		return err
	}

	if config != nil {
		// mutating webhoook configuration already exists
		return nil
	}

	if err := rww.webhookRegistrationClient.CreateResourceMutatingWebhookConfiguration(); err != nil {
		return err
	}
	glog.V(3).Info("Successfully created mutating webhook configuration for resources")
	return nil
}
