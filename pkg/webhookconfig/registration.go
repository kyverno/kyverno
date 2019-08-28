package webhookconfig

import (
	"errors"
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/tevino/abool"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admregclient "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	rest "k8s.io/client-go/rest"
)

// WebhookRegistrationClient is client for registration webhooks on cluster
type WebhookRegistrationClient struct {
	registrationClient *admregclient.AdmissionregistrationV1beta1Client
	client             *client.Client
	clientConfig       *rest.Config
	// serverIP should be used if running Kyverno out of clutser
	serverIP             string
	timeoutSeconds       int32
	MutationRegistered   *abool.AtomicBool
	ValidationRegistered *abool.AtomicBool
}

// NewWebhookRegistrationClient creates new WebhookRegistrationClient instance
func NewWebhookRegistrationClient(clientConfig *rest.Config, client *client.Client, serverIP string, webhookTimeout int32) (*WebhookRegistrationClient, error) {
	registrationClient, err := admregclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	glog.V(3).Infof("Registering webhook client using serverIP %s\n", serverIP)

	return &WebhookRegistrationClient{
		registrationClient:   registrationClient,
		client:               client,
		clientConfig:         clientConfig,
		serverIP:             serverIP,
		timeoutSeconds:       webhookTimeout,
		MutationRegistered:   abool.New(),
		ValidationRegistered: abool.New(),
	}, nil
}

// Register creates admission webhooks configs on cluster
func (wrc *WebhookRegistrationClient) Register() error {
	if wrc.serverIP != "" {
		glog.Infof("Registering webhook with url https://%s\n", wrc.serverIP)
	}

	// For the case if cluster already has this configs
	// remove previously create webhookconfigurations if any
	// webhook configurations are created dynamically based on the policy resources
	wrc.removeWebhookConfigurations()

	// Static Webhook configuration on Policy CRD
	// create Policy CRD validating webhook configuration resource
	// used for validating Policy CR
	if err := wrc.createPolicyValidatingWebhookConfiguration(); err != nil {
		return err
	}
	// create Policy CRD validating webhook configuration resource
	// used for defauling values in Policy CR
	if err := wrc.createPolicyMutatingWebhookConfiguration(); err != nil {
		return err
	}
	return nil
}

// RemovePolicyWebhookConfigurations removes webhook configurations for reosurces and policy
// called during webhook server shutdown
func (wrc *WebhookRegistrationClient) RemovePolicyWebhookConfigurations(cleanUp chan<- struct{}) {
	//TODO: dupliate, but a placeholder to perform more error handlind during cleanup
	wrc.removeWebhookConfigurations()
	// close channel to notify cleanup is complete
	close(cleanUp)
}

func (wrc *WebhookRegistrationClient) CreateResourceMutatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.MutatingWebhookConfiguration

	// read CA data from
	// 1) secret(config)
	// 2) kubeconfig
	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}
	// if serverIP is specified we assume its debug mode
	if wrc.serverIP != "" {
		// debug mode
		// clientConfig - URL
		config = wrc.contructDebugMutatingWebhookConfig(caData)
	} else {
		// clientConfig - service
		config = wrc.constructMutatingWebhookConfig(caData)
	}

	if _, err := wrc.registrationClient.MutatingWebhookConfigurations().Create(config); err != nil {
		return err
	}

	wrc.MutationRegistered.Set()
	return nil
}

func (wrc *WebhookRegistrationClient) CreateResourceValidatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.ValidatingWebhookConfiguration

	// read CA data from
	// 1) secret(config)
	// 2) kubeconfig
	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}
	// if serverIP is specified we assume its debug mode
	if wrc.serverIP != "" {
		// debug mode
		// clientConfig - URL
		config = wrc.contructDebugValidatingWebhookConfig(caData)
	} else {
		// clientConfig - service
		config = wrc.constructValidatingWebhookConfig(caData)
	}
	if _, err := wrc.registrationClient.ValidatingWebhookConfigurations().Create(config); err != nil {
		return err
	}

	wrc.ValidationRegistered.Set()
	return nil
}

//registerPolicyValidatingWebhookConfiguration create a Validating webhook configuration for Policy CRD
func (wrc *WebhookRegistrationClient) createPolicyValidatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.ValidatingWebhookConfiguration

	// read CA data from
	// 1) secret(config)
	// 2) kubeconfig
	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	// if serverIP is specified we assume its debug mode
	if wrc.serverIP != "" {
		// debug mode
		// clientConfig - URL
		config = wrc.contructDebugPolicyValidatingWebhookConfig(caData)
	} else {
		// clientConfig - service
		config = wrc.contructPolicyValidatingWebhookConfig(caData)
	}

	// create validating webhook configuration resource
	if _, err := wrc.registrationClient.ValidatingWebhookConfigurations().Create(config); err != nil {
		return err
	}

	glog.V(4).Infof("created Validating Webhook Configuration %s ", config.Name)
	return nil
}

func (wrc *WebhookRegistrationClient) createPolicyMutatingWebhookConfiguration() error {
	var caData []byte
	var config *admregapi.MutatingWebhookConfiguration

	// read CA data from
	// 1) secret(config)
	// 2) kubeconfig
	if caData = wrc.readCaData(); caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	// if serverIP is specified we assume its debug mode
	if wrc.serverIP != "" {
		// debug mode
		// clientConfig - URL
		config = wrc.contructDebugPolicyMutatingWebhookConfig(caData)
	} else {
		// clientConfig - service
		config = wrc.contructPolicyMutatingWebhookConfig(caData)
	}

	// create mutating webhook configuration resource
	if _, err := wrc.registrationClient.MutatingWebhookConfigurations().Create(config); err != nil {
		return err
	}

	glog.V(4).Infof("created Mutating Webhook Configuration %s ", config.Name)
	return nil
}

// DeregisterAll deletes webhook configs from cluster
// This function does not fail on error:
// Register will fail if the config exists, so there is no need to fail on error
func (wrc *WebhookRegistrationClient) removeWebhookConfigurations() {
	startTime := time.Now()
	glog.V(4).Infof("Started cleaning up webhookconfigurations")
	defer func() {
		glog.V(4).Infof("Finished cleaning up webhookcongfigurations (%v)", time.Since(startTime))
	}()
	// mutating and validating webhook configuration for Kubernetes resources
	wrc.RemoveResourceMutatingWebhookConfiguration()
	wrc.removeResourceValidatingWebhookConfiguration()

	// mutating and validating webhook configurtion for Policy CRD resource
	wrc.removePolicyWebhookConfigurations()
}

// removePolicyWebhookConfigurations removes mutating and validating webhook configurations, if already presnt
// webhookConfigurations are re-created later
func (wrc *WebhookRegistrationClient) removePolicyWebhookConfigurations() {
	// Validating webhook configuration
	var validatingConfig string
	if wrc.serverIP != "" {
		validatingConfig = config.PolicyValidatingWebhookConfigurationDebugName
	} else {
		validatingConfig = config.PolicyValidatingWebhookConfigurationName
	}
	glog.V(4).Infof("removing webhook configuration %s", validatingConfig)
	err := wrc.registrationClient.ValidatingWebhookConfigurations().Delete(validatingConfig, &v1.DeleteOptions{})
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	}

	// Mutating webhook configuration
	var mutatingConfig string
	if wrc.serverIP != "" {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationName
	}

	glog.V(4).Infof("removing webhook configuration %s", mutatingConfig)
	if err := wrc.registrationClient.MutatingWebhookConfigurations().Delete(mutatingConfig, &v1.DeleteOptions{}); err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	}
}

//RemoveResourceMutatingWebhookConfiguration removes mutating webhook configuration for all resources
func (wrc *WebhookRegistrationClient) RemoveResourceMutatingWebhookConfiguration() {
	var configName string
	if wrc.serverIP != "" {
		configName = config.MutatingWebhookConfigurationDebug
	} else {
		configName = config.MutatingWebhookConfigurationName
	}
	// delete webhook configuration
	err := wrc.registrationClient.MutatingWebhookConfigurations().Delete(configName, &v1.DeleteOptions{})
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	} else {
		wrc.MutationRegistered.UnSet()
	}
}

// removeResourceValidatingWebhookConfiguration removes validating webhook configuration on all resources
func (wrc *WebhookRegistrationClient) removeResourceValidatingWebhookConfiguration() {
	var configName string
	if wrc.serverIP != "" {
		configName = config.ValidatingWebhookConfigurationDebug
	} else {
		configName = config.ValidatingWebhookConfigurationName
	}

	err := wrc.registrationClient.ValidatingWebhookConfigurations().Delete(configName, &v1.DeleteOptions{})
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	}
	wrc.ValidationRegistered.UnSet()
}
