package webhookconfig

import (
	"errors"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
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
	serverIP       string
	timeoutSeconds int32
}

// NewWebhookRegistrationClient creates new WebhookRegistrationClient instance
func NewWebhookRegistrationClient(clientConfig *rest.Config, client *client.Client, serverIP string, webhookTimeout int32) (*WebhookRegistrationClient, error) {
	registrationClient, err := admregclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	glog.V(4).Infof("Registering webhook client using serverIP %s\n", serverIP)

	return &WebhookRegistrationClient{
		registrationClient: registrationClient,
		client:             client,
		clientConfig:       clientConfig,
		serverIP:           serverIP,
		timeoutSeconds:     webhookTimeout,
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

	// create Verify mutating webhook configuration resource
	// that is used to check if admission control is enabled or not
	if err := wrc.createVerifyMutatingWebhookConfiguration(); err != nil {
		return err
	}

	return nil
}

// RemoveWebhookConfigurations removes webhook configurations for reosurces and policy
// called during webhook server shutdown
func (wrc *WebhookRegistrationClient) RemoveWebhookConfigurations(cleanUp chan<- struct{}) {
	//TODO: dupliate, but a placeholder to perform more error handlind during cleanup
	wrc.removeWebhookConfigurations()
	// close channel to notify cleanup is complete
	close(cleanUp)
}

//CreateResourceMutatingWebhookConfiguration create a Mutatingwebhookconfiguration resource for all resource type
// used to forward request to kyverno webhooks to apply policeis
// Mutationg webhook is be used for Mutating & Validating purpose
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

	_, err := wrc.registrationClient.MutatingWebhookConfigurations().Create(config)
	if errorsapi.IsAlreadyExists(err) {
		glog.V(4).Infof("resource mutating webhook configuration %s, already exists. not creating one", config.Name)
		return nil
	}
	if err != nil {
		glog.V(4).Infof("failed to create resource mutating webhook configuration %s: %v", config.Name, err)
		return err
	}
	return nil
}

//GetResourceMutatingWebhookConfiguration returns the MutatingWebhookConfiguration
func (wrc *WebhookRegistrationClient) GetResourceMutatingWebhookConfiguration() (*admregapi.MutatingWebhookConfiguration, error) {
	var name string
	if wrc.serverIP != "" {
		name = config.MutatingWebhookConfigurationDebugName
	} else {
		name = config.MutatingWebhookConfigurationName
	}

	return wrc.registrationClient.MutatingWebhookConfigurations().Get(name, v1.GetOptions{})
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

func (wrc *WebhookRegistrationClient) createVerifyMutatingWebhookConfiguration() error {
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
		config = wrc.constructDebugVerifyMutatingWebhookConfig(caData)
	} else {
		// clientConfig - service
		config = wrc.constructVerifyMutatingWebhookConfig(caData)
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

	var wg sync.WaitGroup

	wg.Add(3)
	// mutating and validating webhook configuration for Kubernetes resources
	go wrc.removeResourceMutatingWebhookConfiguration(&wg)
	// mutating and validating webhook configurtion for Policy CRD resource
	go wrc.removePolicyMutatingWebhookConfiguration(&wg)
	go wrc.removePolicyValidatingWebhookConfiguration(&wg)

	// wait for the removal go routines to return
	wg.Wait()
}

// wrapper to handle wait group
// TODO: re-work with RemoveResourceMutatingWebhookConfiguration, as the only difference is wg handling
func (wrc *WebhookRegistrationClient) removeResourceMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	wrc.RemoveResourceMutatingWebhookConfiguration()
}

// delete policy mutating webhookconfigurations
// handle wait group
func (wrc *WebhookRegistrationClient) removePolicyMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	// Mutating webhook configuration
	var mutatingConfig string
	if wrc.serverIP != "" {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationName
	}

	glog.V(4).Infof("removing webhook configuration %s", mutatingConfig)
	err := wrc.registrationClient.MutatingWebhookConfigurations().Delete(mutatingConfig, &v1.DeleteOptions{})
	if errorsapi.IsNotFound(err) {
		glog.V(4).Infof("policy webhook configuration %s, does not exits. not deleting", mutatingConfig)
	} else if err != nil {
		glog.Errorf("failed to delete policy webhook configuration %s: %v", mutatingConfig, err)
	} else {
		glog.V(4).Infof("succesfully deleted policy webhook configuration %s", mutatingConfig)
	}
}

// delete policy validating webhookconfigurations
// handle wait group
func (wrc *WebhookRegistrationClient) removePolicyValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	// Validating webhook configuration
	var err error
	var validatingConfig string
	if wrc.serverIP != "" {
		validatingConfig = config.PolicyValidatingWebhookConfigurationDebugName
	} else {
		validatingConfig = config.PolicyValidatingWebhookConfigurationName
	}
	glog.V(4).Infof("removing webhook configuration %s", validatingConfig)
	err = wrc.registrationClient.ValidatingWebhookConfigurations().Delete(validatingConfig, &v1.DeleteOptions{})
	if errorsapi.IsNotFound(err) {
		glog.V(4).Infof("policy webhook configuration %s, does not exits. not deleting", validatingConfig)
	} else if err != nil {
		glog.Errorf("failed to delete policy webhook configuration %s: %v", validatingConfig, err)
	} else {
		glog.V(4).Infof("succesfully deleted policy webhook configuration %s", validatingConfig)
	}
}
