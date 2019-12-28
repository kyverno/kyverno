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
	vwebhookconfigSynced cache.InformerSynced
	// list/get mutatingwebhookconfigurations
	mWebhookConfigLister           mconfiglister.MutatingWebhookConfigurationLister
	vWebhookConfigLister           mconfiglister.ValidatingWebhookConfigurationLister
	webhookRegistrationClient      *WebhookRegistrationClient
	RunValidationInMutatingWebhook string
}

func NewResourceWebhookRegister(
	lastReqTime *checker.LastReqTime,
	mconfigwebhookinformer mconfiginformer.MutatingWebhookConfigurationInformer,
	vconfigwebhookinformer mconfiginformer.ValidatingWebhookConfigurationInformer,
	webhookRegistrationClient *WebhookRegistrationClient,
	runValidationInMutatingWebhook string,
) *ResourceWebhookRegister {
	return &ResourceWebhookRegister{
		pendingCreation:                abool.New(),
		LastReqTime:                    lastReqTime,
		mwebhookconfigSynced:           mconfigwebhookinformer.Informer().HasSynced,
		mWebhookConfigLister:           mconfigwebhookinformer.Lister(),
		vwebhookconfigSynced:           vconfigwebhookinformer.Informer().HasSynced,
		vWebhookConfigLister:           vconfigwebhookinformer.Lister(),
		webhookRegistrationClient:      webhookRegistrationClient,
		RunValidationInMutatingWebhook: runValidationInMutatingWebhook,
	}
}

func (rww *ResourceWebhookRegister) RegisterResourceWebhook() {
	// drop the request if creation is in processing
	if rww.pendingCreation.IsSet() {
		glog.V(3).Info("resource webhook configuration is in pending creation, skip the request")
		return
	}

	timeDiff := time.Since(rww.LastReqTime.Time())
	if timeDiff < checker.DefaultDeadline {
		glog.V(3).Info("Verified webhook status, creating webhook configuration")
		go func() {
			mutatingConfigName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
			mutatingConfig, _ := rww.mWebhookConfigLister.Get(mutatingConfigName)
			if mutatingConfig != nil {
				glog.V(4).Info("mutating webhoook configuration already exists")
			} else {
				rww.pendingCreation.Set()
				err1 := rww.webhookRegistrationClient.CreateResourceMutatingWebhookConfiguration()
				rww.pendingCreation.UnSet()
				if err1 != nil {
					glog.Errorf("failed to create resource mutating webhook configuration: %v, re-queue creation request", err1)
					rww.RegisterResourceWebhook()
					return
				}
				glog.V(3).Info("Successfully created mutating webhook configuration for resources")
			}

			if rww.RunValidationInMutatingWebhook != "true" {
				validatingConfigName := rww.webhookRegistrationClient.GetResourceValidatingWebhookConfigName()
				validatingConfig, _ := rww.vWebhookConfigLister.Get(validatingConfigName)
				if validatingConfig != nil {
					glog.V(4).Info("validating webhoook configuration already exists")
				} else {
					rww.pendingCreation.Set()
					err2 := rww.webhookRegistrationClient.CreateResourceValidatingWebhookConfiguration()
					rww.pendingCreation.UnSet()
					if err2 != nil {
						glog.Errorf("failed to create resource validating webhook configuration: %v, re-queue creation request", err2)
						rww.RegisterResourceWebhook()
						return
					}
					glog.V(3).Info("Successfully created validating webhook configuration for resources")
				}
			}
		}()
	}
}

func (rww *ResourceWebhookRegister) Run(stopCh <-chan struct{}) {
	// wait for cache to populate first time
	if !cache.WaitForCacheSync(stopCh, rww.mwebhookconfigSynced) {
		glog.Error("configuration: failed to sync webhook informer cache")
	}
}

func (rww *ResourceWebhookRegister) RemoveResourceWebhookConfiguration() error {
	mutatingConfigName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
	mutatingConfig, err := rww.mWebhookConfigLister.Get(mutatingConfigName)
	if err != nil {
		glog.V(4).Infof("failed to list mutating webhook config: %v", err)
		return err
	}
	if mutatingConfig != nil {
		err = rww.webhookRegistrationClient.RemoveResourceMutatingWebhookConfiguration()
		if err != nil {
			return err
		}
		glog.V(3).Info("removed mutating resource webhook configuration")
	}

	if rww.RunValidationInMutatingWebhook != "true" {
		validatingConfigName := rww.webhookRegistrationClient.GetResourceValidatingWebhookConfigName()
		validatingConfig, err := rww.vWebhookConfigLister.Get(validatingConfigName)
		if err != nil {
			glog.V(4).Infof("failed to list validating webhook config: %v", err)
			return err
		}
		if validatingConfig != nil {
			err = rww.webhookRegistrationClient.RemoveResourceValidatingWebhookConfiguration()
			if err != nil {
				return err
			}
			glog.V(3).Info("removed validating resource webhook configuration")
		}
	}
	return nil
}
