package webhookconfig

import (
	"time"

	"github.com/golang/glog"
	checker "github.com/nirmata/kyverno/pkg/checker"
	"github.com/tevino/abool"
	mconfiginformer "k8s.io/client-go/informers/admissionregistration/v1beta1"
	mconfiglister "k8s.io/client-go/listers/admissionregistration/v1beta1"
	cache "k8s.io/client-go/tools/cache"
)

type ResourceWebhookRegister struct {
	// pendingCreation indicates the status of resource webhook creation
	pendingCreation      *abool.AtomicBool
	LastReqTime          *checker.LastReqTime
	mwebhookconfigSynced cache.InformerSynced
	// list/get mutatingwebhookconfigurations
	mWebhookConfigLister      mconfiglister.MutatingWebhookConfigurationLister
	webhookRegistrationClient *WebhookRegistrationClient
}

func NewResourceWebhookRegister(
	lastReqTime *checker.LastReqTime,
	mconfigwebhookinformer mconfiginformer.MutatingWebhookConfigurationInformer,
	webhookRegistrationClient *WebhookRegistrationClient,
) *ResourceWebhookRegister {
	return &ResourceWebhookRegister{
		pendingCreation:           abool.New(),
		LastReqTime:               lastReqTime,
		mwebhookconfigSynced:      mconfigwebhookinformer.Informer().HasSynced,
		mWebhookConfigLister:      mconfigwebhookinformer.Lister(),
		webhookRegistrationClient: webhookRegistrationClient,
	}
}

func (rww *ResourceWebhookRegister) RegisterResourceWebhook() {
	// drop the request if creation is in processing
	if rww.pendingCreation.IsSet() {
		glog.V(3).Info("resource webhook configuration is in pending creation, skip the request")
		return
	}

	// check cache
	configName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
	// exsitence of config is all that matters; if error occurs, creates webhook anyway
	// errors of webhook creation are handled separately
	config, _ := rww.mWebhookConfigLister.Get(configName)
	if config != nil {
		glog.V(4).Info("mutating webhoook configuration already exists, skip the request")
		return
	}

	createWebhook := func() {
		rww.pendingCreation.Set()
		err := rww.webhookRegistrationClient.CreateResourceMutatingWebhookConfiguration()
		rww.pendingCreation.UnSet()

		if err != nil {
			glog.Errorf("failed to create resource mutating webhook configuration: %v, re-queue creation request", err)
			rww.RegisterResourceWebhook()
			return
		}
		glog.V(3).Info("Successfully created mutating webhook configuration for resources")
	}

	timeDiff := time.Since(rww.LastReqTime.Time())
	if timeDiff < checker.DefaultDeadline {
		glog.V(3).Info("Verified webhook status, creating webhook configuration")
		go createWebhook()
	}
}

func (rww *ResourceWebhookRegister) Run(stopCh <-chan struct{}) {
	// wait for cache to populate first time
	if !cache.WaitForCacheSync(stopCh, rww.mwebhookconfigSynced) {
		glog.Error("configuration: failed to sync webhook informer cache")
	}
}

func (rww *ResourceWebhookRegister) RemoveResourceWebhookConfiguration() error {
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
