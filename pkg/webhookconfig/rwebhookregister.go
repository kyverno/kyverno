package webhookconfig

import (
	"time"

	"github.com/go-logr/logr"
	checker "github.com/nirmata/kyverno/pkg/checker"
	"github.com/tevino/abool"
	mconfiginformer "k8s.io/client-go/informers/admissionregistration/v1beta1"
	mconfiglister "k8s.io/client-go/listers/admissionregistration/v1beta1"
	cache "k8s.io/client-go/tools/cache"
)

//ResourceWebhookRegister manages the resource webhook registration
type ResourceWebhookRegister struct {
	// pendingCreation indicates the status of resource webhook creation
	pendingMutateWebhookCreation   *abool.AtomicBool
	pendingValidateWebhookCreation *abool.AtomicBool
	LastReqTime                    *checker.LastReqTime
	mwebhookconfigSynced           cache.InformerSynced
	vwebhookconfigSynced           cache.InformerSynced
	mWebhookConfigLister           mconfiglister.MutatingWebhookConfigurationLister
	vWebhookConfigLister           mconfiglister.ValidatingWebhookConfigurationLister
	webhookRegistrationClient      *WebhookRegistrationClient
	RunValidationInMutatingWebhook string
	log                            logr.Logger
}

// NewResourceWebhookRegister returns a new instance of ResourceWebhookRegister manager
func NewResourceWebhookRegister(
	lastReqTime *checker.LastReqTime,
	mconfigwebhookinformer mconfiginformer.MutatingWebhookConfigurationInformer,
	vconfigwebhookinformer mconfiginformer.ValidatingWebhookConfigurationInformer,
	webhookRegistrationClient *WebhookRegistrationClient,
	runValidationInMutatingWebhook string,
	log logr.Logger,
) *ResourceWebhookRegister {
	return &ResourceWebhookRegister{
		pendingMutateWebhookCreation:   abool.New(),
		pendingValidateWebhookCreation: abool.New(),
		LastReqTime:                    lastReqTime,
		mwebhookconfigSynced:           mconfigwebhookinformer.Informer().HasSynced,
		mWebhookConfigLister:           mconfigwebhookinformer.Lister(),
		vwebhookconfigSynced:           vconfigwebhookinformer.Informer().HasSynced,
		vWebhookConfigLister:           vconfigwebhookinformer.Lister(),
		webhookRegistrationClient:      webhookRegistrationClient,
		RunValidationInMutatingWebhook: runValidationInMutatingWebhook,
		log:                            log,
	}
}

//RegisterResourceWebhook registers a resource webhook
func (rww *ResourceWebhookRegister) RegisterResourceWebhook() {
	timeDiff := time.Since(rww.LastReqTime.Time())
	if timeDiff < checker.DefaultDeadline {
		if !rww.pendingMutateWebhookCreation.IsSet() {
			go rww.createMutatingWebhook()
		}

		if !rww.pendingValidateWebhookCreation.IsSet() {
			go rww.createValidateWebhook()
		}
	}
}

func (rww *ResourceWebhookRegister) createMutatingWebhook() {
	rww.pendingMutateWebhookCreation.Set()
	defer rww.pendingMutateWebhookCreation.UnSet()

	mutatingConfigName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
	mutatingConfig, _ := rww.mWebhookConfigLister.Get(mutatingConfigName)
	if mutatingConfig != nil {
		rww.log.V(5).Info("mutating webhoook configuration exists", "name", mutatingConfigName)
	} else {
		err := rww.webhookRegistrationClient.CreateResourceMutatingWebhookConfiguration()
		if err != nil {
			rww.log.Error(err, "failed to create resource mutating webhook configuration, re-queue creation request")
			rww.RegisterResourceWebhook()
			return
		}

		rww.log.V(2).Info("created mutating webhook", "name", mutatingConfigName)
	}
}

func (rww *ResourceWebhookRegister) createValidateWebhook() {
	rww.pendingValidateWebhookCreation.Set()
	defer rww.pendingValidateWebhookCreation.UnSet()

	if rww.RunValidationInMutatingWebhook == "true" {
		rww.log.V(2).Info("validation is configured to run during mutate webhook")
		return
	}

	validatingConfigName := rww.webhookRegistrationClient.GetResourceValidatingWebhookConfigName()
	validatingConfig, _ := rww.vWebhookConfigLister.Get(validatingConfigName)
	if validatingConfig != nil {
		rww.log.V(4).Info("validating webhoook configuration exists", "name", validatingConfigName)
	} else {
		err := rww.webhookRegistrationClient.CreateResourceValidatingWebhookConfiguration()
		if err != nil {
			rww.log.Error(err, "failed to create resource validating webhook configuration; re-queue creation request")
			rww.RegisterResourceWebhook()
			return
		}

		rww.log.V(2).Info("created validating webhook", "name", validatingConfigName)
	}
}

//Run starts the ResourceWebhookRegister manager
func (rww *ResourceWebhookRegister) Run(stopCh <-chan struct{}) {
	logger := rww.log
	// wait for cache to populate first time
	if !cache.WaitForCacheSync(stopCh, rww.mwebhookconfigSynced, rww.vwebhookconfigSynced) {
		logger.Info("configuration: failed to sync webhook informer cache")
	}
}

// RemoveResourceWebhookConfiguration removes the resource webhook configurations
func (rww *ResourceWebhookRegister) RemoveResourceWebhookConfiguration() error {
	logger := rww.log
	mutatingConfigName := rww.webhookRegistrationClient.GetResourceMutatingWebhookConfigName()
	mutatingConfig, err := rww.mWebhookConfigLister.Get(mutatingConfigName)
	if err != nil {
		logger.Error(err, "failed to list mutating webhook config")
		return err
	}
	if mutatingConfig != nil {
		err = rww.webhookRegistrationClient.RemoveResourceMutatingWebhookConfiguration()
		if err != nil {
			return err
		}
		logger.V(3).Info("removed mutating resource webhook configuration")
	}

	if rww.RunValidationInMutatingWebhook != "true" {
		validatingConfigName := rww.webhookRegistrationClient.GetResourceValidatingWebhookConfigName()
		validatingConfig, err := rww.vWebhookConfigLister.Get(validatingConfigName)
		if err != nil {
			logger.Error(err, "failed to list validating webhook config")
			return err
		}
		if validatingConfig != nil {
			err = rww.webhookRegistrationClient.RemoveResourceValidatingWebhookConfiguration()
			if err != nil {
				return err
			}
			logger.V(3).Info("removed validating resource webhook configuration")
		}
	}
	return nil
}
