package webhooks

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

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
	wrc.DeregisterAll()

	// register policy validating webhook during inital start
	return wrc.RegisterPolicyValidatingWebhook()
}

func (wrc *WebhookRegistrationClient) RegisterMutatingWebhook() error {
	mutatingWebhookConfig, err := wrc.constructMutatingWebhookConfig(wrc.clientConfig)
	if err != nil {
		return err
	}

	if _, err = wrc.registrationClient.MutatingWebhookConfigurations().Create(mutatingWebhookConfig); err != nil {
		return err
	}

	wrc.MutationRegistered.Set()
	return nil
}

func (wrc *WebhookRegistrationClient) RegisterValidatingWebhook() error {
	validationWebhookConfig, err := wrc.constructValidatingWebhookConfig(wrc.clientConfig)
	if err != nil {
		return err
	}

	if _, err = wrc.registrationClient.ValidatingWebhookConfigurations().Create(validationWebhookConfig); err != nil {
		return err
	}

	wrc.ValidationRegistered.Set()
	return nil
}

func (wrc *WebhookRegistrationClient) RegisterPolicyValidatingWebhook() error {
	policyValidationWebhookConfig, err := wrc.contructPolicyValidatingWebhookConfig()
	if err != nil {
		return err
	}

	if _, err = wrc.registrationClient.ValidatingWebhookConfigurations().Create(policyValidationWebhookConfig); err != nil {
		return err
	}

	glog.V(3).Infoln("Policy validating webhook registered")
	return nil
}

// DeregisterAll deletes webhook configs from cluster
// This function does not fail on error:
// Register will fail if the config exists, so there is no need to fail on error
func (wrc *WebhookRegistrationClient) DeregisterAll() {
	wrc.deregisterMutatingWebhook()
	wrc.deregisterValidatingWebhook()

	if wrc.serverIP != "" {
		err := wrc.registrationClient.ValidatingWebhookConfigurations().Delete(config.PolicyValidatingWebhookConfigurationDebug, &v1.DeleteOptions{})
		if err != nil && !errorsapi.IsNotFound(err) {
			glog.Error(err)
		}
	}
	err := wrc.registrationClient.ValidatingWebhookConfigurations().Delete(config.PolicyValidatingWebhookConfigurationName, &v1.DeleteOptions{})
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	}
}

func (wrc *WebhookRegistrationClient) deregister() {
	wrc.deregisterMutatingWebhook()
	wrc.deregisterValidatingWebhook()
}

func (wrc *WebhookRegistrationClient) deregisterMutatingWebhook() {
	if wrc.serverIP != "" {
		err := wrc.registrationClient.MutatingWebhookConfigurations().Delete(config.MutatingWebhookConfigurationDebug, &v1.DeleteOptions{})
		if err != nil && !errorsapi.IsNotFound(err) {
			glog.Error(err)
		} else {
			wrc.MutationRegistered.UnSet()
		}
		return
	}

	err := wrc.registrationClient.MutatingWebhookConfigurations().Delete(config.MutatingWebhookConfigurationName, &v1.DeleteOptions{})
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	} else {
		wrc.MutationRegistered.UnSet()
	}
}

func (wrc *WebhookRegistrationClient) deregisterValidatingWebhook() {
	if wrc.serverIP != "" {
		err := wrc.registrationClient.ValidatingWebhookConfigurations().Delete(config.ValidatingWebhookConfigurationDebug, &v1.DeleteOptions{})
		if err != nil && !errorsapi.IsNotFound(err) {
			glog.Error(err)
		}
		wrc.ValidationRegistered.UnSet()
		return
	}

	err := wrc.registrationClient.ValidatingWebhookConfigurations().Delete(config.ValidatingWebhookConfigurationName, &v1.DeleteOptions{})
	if err != nil && !errorsapi.IsNotFound(err) {
		glog.Error(err)
	}
	wrc.ValidationRegistered.UnSet()
}

func (wrc *WebhookRegistrationClient) constructMutatingWebhookConfig(configuration *rest.Config) (*admregapi.MutatingWebhookConfiguration, error) {
	var caData []byte
	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	caData = wrc.client.ReadRootCASecret()
	if len(caData) == 0 {
		// load the CA from kubeconfig
		caData = extractCA(configuration)
	}
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		return wrc.contructDebugMutatingWebhookConfig(caData), nil
	}

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.MutatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			constructWebhook(
				config.MutatingWebhookName,
				config.MutatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
			),
		},
	}, nil
}

func (wrc *WebhookRegistrationClient) contructDebugMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.MutatingWebhookServicePath)
	glog.V(3).Infof("Debug MutatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.MutatingWebhookConfigurationDebug,
			Labels: config.KubePolicyAppLabels,
		},
		Webhooks: []admregapi.Webhook{
			constructDebugWebhook(
				config.MutatingWebhookName,
				url,
				caData,
				false,
				wrc.timeoutSeconds),
		},
	}
}

func (wrc *WebhookRegistrationClient) constructValidatingWebhookConfig(configuration *rest.Config) (*admregapi.ValidatingWebhookConfiguration, error) {
	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	caData := wrc.client.ReadRootCASecret()
	if len(caData) == 0 {
		// load the CA from kubeconfig
		caData = extractCA(configuration)
	}
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		return wrc.contructDebugValidatingWebhookConfig(caData), nil
	}

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.ValidatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			constructWebhook(
				config.ValidatingWebhookName,
				config.ValidatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds),
		},
	}, nil
}

func (wrc *WebhookRegistrationClient) contructDebugValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.ValidatingWebhookServicePath)
	glog.V(3).Infof("Debug ValidatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.ValidatingWebhookConfigurationDebug,
			Labels: config.KubePolicyAppLabels,
		},
		Webhooks: []admregapi.Webhook{
			constructDebugWebhook(
				config.ValidatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds),
		},
	}
}

func (wrc *WebhookRegistrationClient) contructPolicyValidatingWebhookConfig() (*admregapi.ValidatingWebhookConfiguration, error) {
	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	caData := wrc.client.ReadRootCASecret()
	if len(caData) == 0 {
		// load the CA from kubeconfig
		caData = extractCA(wrc.clientConfig)
	}
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	if wrc.serverIP != "" {
		return wrc.contructDebugPolicyValidatingWebhookConfig(caData), nil
	}

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.PolicyValidatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			constructWebhook(
				config.PolicyValidatingWebhookName,
				config.PolicyValidatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds),
		},
	}, nil
}

func (wrc *WebhookRegistrationClient) contructDebugPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.PolicyValidatingWebhookServicePath)
	glog.V(3).Infof("Debug PolicyValidatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.PolicyValidatingWebhookConfigurationDebug,
			Labels: config.KubePolicyAppLabels,
		},
		Webhooks: []admregapi.Webhook{
			constructDebugWebhook(
				config.PolicyValidatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds),
		},
	}
}

func constructWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32) admregapi.Webhook {
	resource := "*/*"
	apiGroups := "*"
	apiversions := "*"
	if servicePath == config.PolicyValidatingWebhookServicePath {
		resource = "policies/*"
		apiGroups = "kyverno.io"
		apiversions = "v1alpha1"
	}
	operationtypes := []admregapi.OperationType{
		admregapi.Create,
		admregapi.Update,
	}
	// Add operation DELETE for validation
	if validation {
		operationtypes = append(operationtypes, admregapi.Delete)

	}

	return admregapi.Webhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			Service: &admregapi.ServiceReference{
				Namespace: config.KubePolicyNamespace,
				Name:      config.WebhookServiceName,
				Path:      &servicePath,
			},
			CABundle: caData,
		},
		Rules: []admregapi.RuleWithOperations{
			admregapi.RuleWithOperations{
				Operations: operationtypes,
				Rule: admregapi.Rule{
					APIGroups: []string{
						apiGroups,
					},
					APIVersions: []string{
						apiversions,
					},
					Resources: []string{
						resource,
					},
				},
			},
		},
		TimeoutSeconds: &timeoutSeconds,
	}
}

func constructDebugWebhook(name, url string, caData []byte, validation bool, timeoutSeconds int32) admregapi.Webhook {
	resource := "*/*"
	apiGroups := "*"
	apiversions := "*"

	if strings.Contains(url, config.PolicyValidatingWebhookServicePath) {
		resource = "policies/*"
		apiGroups = "kyverno.io"
		apiversions = "v1alpha1"
	}
	operationtypes := []admregapi.OperationType{
		admregapi.Create,
		admregapi.Update,
	}
	// Add operation DELETE for validation
	if validation {
		operationtypes = append(operationtypes, admregapi.Delete)
	}

	return admregapi.Webhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			URL:      &url,
			CABundle: caData,
		},
		Rules: []admregapi.RuleWithOperations{
			admregapi.RuleWithOperations{
				Operations: operationtypes,
				Rule: admregapi.Rule{
					APIGroups: []string{
						apiGroups,
					},
					APIVersions: []string{
						apiversions,
					},
					Resources: []string{
						resource,
					},
				},
			},
		},
		TimeoutSeconds: &timeoutSeconds,
	}
}

func (wrc *WebhookRegistrationClient) constructOwner() v1.OwnerReference {
	kubePolicyDeployment, err := wrc.client.GetKubePolicyDeployment()

	if err != nil {
		glog.Errorf("Error when constructing OwnerReference, err: %v\n", err)
		return v1.OwnerReference{}
	}

	return v1.OwnerReference{
		APIVersion: config.DeploymentAPIVersion,
		Kind:       config.DeploymentKind,
		Name:       kubePolicyDeployment.ObjectMeta.Name,
		UID:        kubePolicyDeployment.ObjectMeta.UID,
	}
}

// ExtractCA used for extraction CA from config
func extractCA(config *rest.Config) (result []byte) {
	fileName := config.TLSClientConfig.CAFile

	if fileName != "" {
		result, err := ioutil.ReadFile(fileName)

		if err != nil {
			return nil
		}

		return result
	}

	return config.TLSClientConfig.CAData
}
