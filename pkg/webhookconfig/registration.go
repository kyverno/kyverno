package webhookconfig

import (
	"errors"
	"time"

	"github.com/golang/glog"
	client "github.com/nirmata/kyverno/pkg/dclient"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	rest "k8s.io/client-go/rest"
)

const (
	MutatingWebhookConfigurationKind   string = "MutatingWebhookConfiguration"
	ValidatingWebhookConfigurationKind string = "ValidatingWebhookConfiguration"
)

// WebhookRegistrationClient is client for registration webhooks on cluster
type WebhookRegistrationClient struct {
	client       *client.Client
	clientConfig *rest.Config
	// serverIP should be used if running Kyverno out of clutser
	serverIP       string
	timeoutSeconds int32
}

// NewWebhookRegistrationClient creates new WebhookRegistrationClient instance
func NewWebhookRegistrationClient(
	clientConfig *rest.Config,
	client *client.Client,
	serverIP string,
	webhookTimeout int32) *WebhookRegistrationClient {
	return &WebhookRegistrationClient{
		clientConfig:   clientConfig,
		client:         client,
		serverIP:       serverIP,
		timeoutSeconds: webhookTimeout,
	}
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

// RemovePolicyWebhookConfigurations removes webhook configurations for reosurces and policy
// called during webhook server shutdown
func (wrc *WebhookRegistrationClient) RemovePolicyWebhookConfigurations(cleanUp chan<- struct{}) {
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
	_, err := wrc.client.CreateResource(MutatingWebhookConfigurationKind, "", *config, false)
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
	if _, err := wrc.client.CreateResource(ValidatingWebhookConfigurationKind, "", *config, false); err != nil {
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
	if _, err := wrc.client.CreateResource(MutatingWebhookConfigurationKind, "", *config, false); err != nil {
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
	if _, err := wrc.client.CreateResource(MutatingWebhookConfigurationKind, "", *config, false); err != nil {
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

	// mutating and validating webhook configurtion for Policy CRD resource
	wrc.removePolicyWebhookConfigurations()

	// muating webhook configuration use to verify if admission control flow is working or not
	wrc.removeVerifyWebhookMutatingWebhookConfig()
}
