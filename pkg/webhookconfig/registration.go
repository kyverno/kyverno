package webhookconfig

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

const (
	kindMutating   string = "MutatingWebhookConfiguration"
	kindValidating string = "ValidatingWebhookConfiguration"
)

// Register manages webhook registration. There are five webhooks:
// 1. Policy Validation
// 2. Policy Mutation
// 3. Resource Validation
// 4. Resource Mutation
// 5. Webhook Status Mutation
type Register struct {
	client         *client.Client
	clientConfig   *rest.Config
	resCache       resourcecache.ResourceCache
	serverIP       string // when running outside a cluster
	timeoutSeconds int32
	log            logr.Logger
}

// NewRegister creates new Register instance
func NewRegister(
	clientConfig *rest.Config,
	client *client.Client,
	resCache resourcecache.ResourceCache,
	serverIP string,
	webhookTimeout int32,
	log logr.Logger) *Register {
	return &Register{
		clientConfig:   clientConfig,
		client:         client,
		resCache:       resCache,
		serverIP:       serverIP,
		timeoutSeconds: webhookTimeout,
		log:            log.WithName("Register"),
	}
}

// Register clean up the old webhooks and re-creates admission webhooks configs on cluster
func (wrc *Register) Register() error {
	logger := wrc.log
	if wrc.serverIP != "" {
		logger.Info("Registering webhook", "url", fmt.Sprintf("https://%s", wrc.serverIP))
	}

	wrc.removeWebhookConfigurations()

	errors := make([]string, 0)
	if err := wrc.createVerifyMutatingWebhookConfiguration(); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createPolicyValidatingWebhookConfiguration(); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createPolicyMutatingWebhookConfiguration(); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createResourceValidatingWebhookConfiguration(); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createResourceMutatingWebhookConfiguration(); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, ","))
	}

	return nil
}

// Check returns an error if any of the webhooks are not configured
func (wrc *Register) Check() error {
	mutatingCache, _ := wrc.resCache.GetGVRCache(kindMutating)
	validatingCache, _ := wrc.resCache.GetGVRCache(kindValidating)

	if _, err := mutatingCache.Lister().Get(wrc.getVerifyWebhookMutatingWebhookName()); err != nil {
		return err
	}

	if _, err := mutatingCache.Lister().Get(wrc.getResourceMutatingWebhookConfigName()); err != nil {
		return err
	}

	if _, err := validatingCache.Lister().Get(wrc.getResourceValidatingWebhookConfigName()); err != nil {
		return err
	}

	if _, err := mutatingCache.Lister().Get(wrc.getPolicyMutatingWebhookConfigurationName()); err != nil {
		return err
	}

	if _, err := validatingCache.Lister().Get(wrc.getPolicyValidatingWebhookConfigurationName()); err != nil {
		return err
	}

	return nil
}

// Remove removes all webhook configurations
func (wrc *Register) Remove(cleanUp chan<- struct{}) {
	wrc.removeWebhookConfigurations()
	wrc.removeSecrets()
	close(cleanUp)
}

func (wrc *Register) createResourceMutatingWebhookConfiguration() error {

	var caData []byte
	var config *admregapi.MutatingWebhookConfiguration

	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		config = wrc.constructDebugMutatingWebhookConfig(caData)
	} else {
		config = wrc.constructMutatingWebhookConfig(caData)
	}

	logger := wrc.log.WithValues("kind", kindMutating, "name", config.Name)

	_, err := wrc.client.CreateResource("", kindMutating, "", *config, false)
	if errorsapi.IsAlreadyExists(err) {
		logger.V(6).Info("resource mutating webhook configuration already exists", "name", config.Name)
		return nil
	}

	if err != nil {
		logger.Error(err, "failed to create resource mutating webhook configuration", "name", config.Name)
		return err
	}

	logger.Info("created webhook")
	return nil
}

func (wrc *Register) createResourceValidatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.ValidatingWebhookConfiguration

	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}
	if wrc.serverIP != "" {
		config = wrc.constructDebugValidatingWebhookConfig(caData)
	} else {
		config = wrc.constructValidatingWebhookConfig(caData)
	}

	logger := wrc.log.WithValues("kind", kindValidating, "name", config.Name)

	_, err := wrc.client.CreateResource("", kindValidating, "", *config, false)
	if errorsapi.IsAlreadyExists(err) {
		logger.V(6).Info("resource validating webhook configuration already exists", "name", config.Name)
		return nil
	}

	if err != nil {
		logger.Error(err, "failed to create resource")
		return err
	}

	logger.Info("created webhook")
	return nil
}

//registerPolicyValidatingWebhookConfiguration create a Validating webhook configuration for Policy CRD
func (wrc *Register) createPolicyValidatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.ValidatingWebhookConfiguration

	// read certificate data
	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		config = wrc.contructDebugPolicyValidatingWebhookConfig(caData)
	} else {
		config = wrc.contructPolicyValidatingWebhookConfig(caData)
	}

	if _, err := wrc.client.CreateResource("", kindValidating, "", *config, false); err != nil {
		if errorsapi.IsAlreadyExists(err) {
			wrc.log.V(6).Info("webhook already exists", "kind", kindValidating, "name", config.Name)
			return nil
		}

		return err
	}

	wrc.log.Info("created webhook", "kind", kindValidating, "name", config.Name)
	return nil
}

func (wrc *Register) createPolicyMutatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.MutatingWebhookConfiguration

	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		config = wrc.contructDebugPolicyMutatingWebhookConfig(caData)
	} else {
		config = wrc.contructPolicyMutatingWebhookConfig(caData)
	}

	// create mutating webhook configuration resource
	if _, err := wrc.client.CreateResource("", kindMutating, "", *config, false); err != nil {
		if errorsapi.IsAlreadyExists(err) {
			wrc.log.V(6).Info("webhook already exists", "kind", kindMutating, "name", config.Name)
			return nil
		}

		return err
	}

	wrc.log.Info("created webhook", "kind", kindMutating, "name", config.Name)
	return nil
}

func (wrc *Register) createVerifyMutatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.MutatingWebhookConfiguration

	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		config = wrc.constructDebugVerifyMutatingWebhookConfig(caData)
	} else {
		config = wrc.constructVerifyMutatingWebhookConfig(caData)
	}

	if _, err := wrc.client.CreateResource("", kindMutating, "", *config, false); err != nil {
		if errorsapi.IsAlreadyExists(err) {
			wrc.log.V(6).Info("webhook already exists", "kind", kindMutating, "name", config.Name)
			return nil
		}

		return err
	}

	wrc.log.Info("created webhook", "kind", kindMutating, "name", config.Name)
	return nil
}

func (wrc *Register) removeWebhookConfigurations() {
	startTime := time.Now()
	wrc.log.Info("deleting all webhook configurations")
	defer func() {
		wrc.log.V(4).Info("removed webhook configurations", "processingTime", time.Since(startTime).String())
	}()

	var wg sync.WaitGroup
	wg.Add(5)

	go wrc.removeResourceMutatingWebhookConfiguration(&wg)
	go wrc.removeResourceValidatingWebhookConfiguration(&wg)
	go wrc.removePolicyMutatingWebhookConfiguration(&wg)
	go wrc.removePolicyValidatingWebhookConfiguration(&wg)
	go wrc.removeVerifyWebhookMutatingWebhookConfig(&wg)

	wg.Wait()
}

func (wrc *Register) removePolicyMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	mutatingConfig := wrc.getPolicyMutatingWebhookConfigurationName()

	logger := wrc.log.WithValues("kind", kindMutating, "name", mutatingConfig)
	err := wrc.client.DeleteResource("", kindMutating, "", mutatingConfig, false)
	if errorsapi.IsNotFound(err) {
		logger.V(5).Info("policy mutating webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete policy mutating webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
}

func (wrc *Register) getPolicyMutatingWebhookConfigurationName() string {
	var mutatingConfig string
	if wrc.serverIP != "" {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationName
	}
	return mutatingConfig
}

func (wrc *Register) removePolicyValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	validatingConfig := wrc.getPolicyValidatingWebhookConfigurationName()

	logger := wrc.log.WithValues("kind", kindValidating, "name", validatingConfig)
	logger.V(4).Info("removing validating webhook configuration")
	err := wrc.client.DeleteResource("", kindValidating, "", validatingConfig, false)
	if errorsapi.IsNotFound(err) {
		logger.V(5).Info("policy validating webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete policy validating webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
}

func (wrc *Register) getPolicyValidatingWebhookConfigurationName() string {
	var validatingConfig string
	if wrc.serverIP != "" {
		validatingConfig = config.PolicyValidatingWebhookConfigurationDebugName
	} else {
		validatingConfig = config.PolicyValidatingWebhookConfigurationName
	}
	return validatingConfig
}

func (wrc *Register) constructVerifyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.VerifyMutatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(
				config.VerifyMutatingWebhookName,
				config.VerifyMutatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				[]string{"deployments/*"},
				"apps",
				"v1",
				[]admregapi.OperationType{admregapi.Update},
			),
		},
	}
}

func (wrc *Register) constructDebugVerifyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	logger := wrc.log
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.VerifyMutatingWebhookServicePath)
	logger.V(4).Info("Debug VerifyMutatingWebhookConfig is registered with url", "url", url)
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.VerifyMutatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateDebugMutatingWebhook(
				config.VerifyMutatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				[]string{"deployments/*"},
				"apps",
				"v1",
				[]admregapi.OperationType{admregapi.Update},
			),
		},
	}
}

func (wrc *Register) removeVerifyWebhookMutatingWebhookConfig(wg *sync.WaitGroup) {
	defer wg.Done()

	var err error
	mutatingConfig := wrc.getVerifyWebhookMutatingWebhookName()

	logger := wrc.log.WithValues("kind", kindMutating, "name", mutatingConfig)
	err = wrc.client.DeleteResource("", kindMutating, "", mutatingConfig, false)
	if errorsapi.IsNotFound(err) {
		logger.V(5).Info("verify webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete verify webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
}

func (wrc *Register) getVerifyWebhookMutatingWebhookName() string {
	var mutatingConfig string
	if wrc.serverIP != "" {
		mutatingConfig = config.VerifyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.VerifyMutatingWebhookConfigurationName
	}
	return mutatingConfig
}

// GetWebhookTimeOut returns the value of webhook timeout
func (wrc *Register) GetWebhookTimeOut() time.Duration {
	return time.Duration(wrc.timeoutSeconds)
}

// removeSecrets removes Kyverno managed secrets
func (wrc *Register) removeSecrets() {
	selector := &v1.LabelSelector{
		MatchLabels: map[string]string{
			client.ManagedByLabel: "kyverno",
		},
	}

	secretList, err := wrc.client.ListResource("", "Secret", config.KyvernoNamespace, selector)
	if err != nil && errorsapi.IsNotFound(err) {
		wrc.log.Error(err, "failed to clean up Kyverno managed secrets")
		return
	}

	for _, secret := range secretList.Items {
		if err := wrc.client.DeleteResource("", "Secret", secret.GetNamespace(), secret.GetName(), false); err != nil {
			wrc.log.Error(err, "failed to delete secret", "ns", secret.GetNamespace(), "name", secret.GetName())
		}
	}
}
