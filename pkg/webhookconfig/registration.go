package webhookconfig

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/pkg/errors"
	admregapi "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	client             *client.Client
	clientConfig       *rest.Config
	resCache           resourcecache.ResourceCache
	serverIP           string // when running outside a cluster
	timeoutSeconds     int32
	log                logr.Logger
	debug              bool
	autoUpdateWebhooks bool

	UpdateWebhookChan    chan bool
	createDefaultWebhook chan string

	// manage implements methods to manage webhook configurations
	manage
}

// NewRegister creates new Register instance
func NewRegister(
	clientConfig *rest.Config,
	client *client.Client,
	kyvernoClient *kyvernoclient.Clientset,
	resCache resourcecache.ResourceCache,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	serverIP string,
	webhookTimeout int32,
	debug bool,
	autoUpdateWebhooks bool,
	stopCh <-chan struct{},
	log logr.Logger) *Register {
	register := &Register{
		clientConfig:         clientConfig,
		client:               client,
		resCache:             resCache,
		serverIP:             serverIP,
		timeoutSeconds:       webhookTimeout,
		log:                  log.WithName("Register"),
		debug:                debug,
		autoUpdateWebhooks:   autoUpdateWebhooks,
		UpdateWebhookChan:    make(chan bool),
		createDefaultWebhook: make(chan string),
	}

	register.manage = newWebhookConfigManager(client, kyvernoClient, pInformer, npInformer, resCache, serverIP, register.autoUpdateWebhooks, register.createDefaultWebhook, stopCh, log.WithName("WebhookConfigManager"))

	return register
}

// Register clean up the old webhooks and re-creates admission webhooks configs on cluster
func (wrc *Register) Register() error {
	logger := wrc.log
	if wrc.serverIP != "" {
		logger.Info("Registering webhook", "url", fmt.Sprintf("https://%s", wrc.serverIP))
	}
	if !wrc.debug {
		if err := wrc.checkEndpoint(); err != nil {
			return err
		}
	}
	wrc.removeWebhookConfigurations()

	caData := wrc.readCaData()
	if caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}

	errors := make([]string, 0)
	if err := wrc.createVerifyMutatingWebhookConfiguration(caData); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createPolicyValidatingWebhookConfiguration(caData); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createPolicyMutatingWebhookConfiguration(caData); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createResourceValidatingWebhookConfiguration(caData); err != nil {
		errors = append(errors, err.Error())
	}

	if err := wrc.createResourceMutatingWebhookConfiguration(caData); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, ","))
	}

	go wrc.manage.start()
	return nil
}

// Check returns an error if any of the webhooks are not configured
func (wrc *Register) Check() error {
	mutatingCache, _ := wrc.resCache.GetGVRCache(kindMutating)
	validatingCache, _ := wrc.resCache.GetGVRCache(kindValidating)

	if _, err := mutatingCache.Lister().Get(wrc.getVerifyWebhookMutatingWebhookName()); err != nil {
		return err
	}

	if _, err := mutatingCache.Lister().Get(getResourceMutatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}

	if _, err := validatingCache.Lister().Get(getResourceValidatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}

	if _, err := mutatingCache.Lister().Get(getPolicyMutatingWebhookConfigurationName(wrc.serverIP)); err != nil {
		return err
	}

	if _, err := validatingCache.Lister().Get(getPolicyValidatingWebhookConfigurationName(wrc.serverIP)); err != nil {
		return err
	}

	return nil
}

// Remove removes all webhook configurations
func (wrc *Register) Remove(cleanUp chan<- struct{}) {
	defer close(cleanUp)
	if !wrc.cleanupKyvernoResource() {
		return
	}

	wrc.removeWebhookConfigurations()
	wrc.removeSecrets()
	err := wrc.client.DeleteResource("coordination.k8s.io/v1", "Lease", config.KyvernoNamespace, "kyvernopre-lock", false)
	if err != nil && errorsapi.IsNotFound(err) {
		wrc.log.WithName("cleanup").Error(err, "failed to clean up Lease lock")
	}

}

// UpdateWebhookConfigurations updates resource webhook configurations dynamically
// base on the UPDATEs of Kyverno init-config ConfigMap
//
// it currently updates namespaceSelector only, can be extend to update other fields
// +deprecated
func (wrc *Register) UpdateWebhookConfigurations(configHandler config.Interface) {
	logger := wrc.log.WithName("UpdateWebhookConfigurations")
	for {
		<-wrc.UpdateWebhookChan
		logger.Info("received the signal to update webhook configurations")

		var nsSelector map[string]interface{}
		webhookCfgs := configHandler.GetWebhooks()
		if webhookCfgs != nil {
			selector := webhookCfgs[0].NamespaceSelector
			selectorBytes, err := json.Marshal(*selector)
			if err != nil {
				logger.Error(err, "failed to serialize namespaceSelector")
				continue
			}

			if err = json.Unmarshal(selectorBytes, &nsSelector); err != nil {
				logger.Error(err, "failed to convert namespaceSelector to the map")
				continue
			}
		}

		if err := wrc.updateResourceMutatingWebhookConfiguration(nsSelector); err != nil {
			logger.Error(err, "unable to update mutatingWebhookConfigurations", "name", getResourceMutatingWebhookConfigName(wrc.serverIP))
			go func() { wrc.UpdateWebhookChan <- true }()
		} else {
			logger.Info("successfully updated mutatingWebhookConfigurations", "name", getResourceMutatingWebhookConfigName(wrc.serverIP))
		}

		if err := wrc.updateResourceValidatingWebhookConfiguration(nsSelector); err != nil {
			logger.Error(err, "unable to update validatingWebhookConfigurations", "name", getResourceValidatingWebhookConfigName(wrc.serverIP))
			go func() { wrc.UpdateWebhookChan <- true }()
		} else {
			logger.Info("successfully updated validatingWebhookConfigurations", "name", getResourceValidatingWebhookConfigName(wrc.serverIP))
		}
	}
}

func (wrc *Register) ValidateWebhookConfigurations(namespace, name string) error {
	logger := wrc.log.WithName("ValidateWebhookConfigurations")

	cm, err := wrc.client.GetResource("", "ConfigMap", namespace, name)
	if err != nil {
		logger.Error(err, "unable to fetch ConfigMap", "namespace", namespace, "name", name)
		return nil
	}

	webhooks, ok, err := unstructured.NestedString(cm.UnstructuredContent(), "data", "webhooks")
	if err != nil {
		logger.Error(err, "failed to fetch tag 'webhooks' from the ConfigMap")
		return nil
	}

	if !ok {
		logger.V(4).Info("webhook configurations not defined")
		return nil
	}

	webhookCfgs := make([]config.WebhookConfig, 0, 10)
	return json.Unmarshal([]byte(webhooks), &webhookCfgs)
}

// cleanupKyvernoResource returns true if Kyverno deployment is terminating
func (wrc *Register) cleanupKyvernoResource() bool {
	logger := wrc.log.WithName("cleanupKyvernoResource")
	deploy, err := wrc.client.GetResource("", "Deployment", deployNamespace, deployName)
	if err != nil {
		logger.Error(err, "failed to get deployment, cleanup kyverno resources anyway")
		return true
	}

	if deploy.GetDeletionTimestamp() != nil {
		logger.Info("Kyverno is terminating, cleanup Kyverno resources")
		return true
	}

	replicas, _, err := unstructured.NestedInt64(deploy.UnstructuredContent(), "spec", "replicas")
	if err != nil {
		logger.Error(err, "unable to fetch spec.replicas of Kyverno deployment")
	}

	if replicas == 0 {
		logger.Info("Kyverno is scaled to zero, cleanup Kyverno resources")
		return true
	}

	logger.Info("updating Kyverno Pod, won't clean up Kyverno resources")
	return false
}

func (wrc *Register) createResourceMutatingWebhookConfiguration(caData []byte) error {
	var config *admregapi.MutatingWebhookConfiguration

	if wrc.serverIP != "" {
		config = wrc.constructDefaultDebugMutatingWebhookConfig(caData)
	} else {
		config = wrc.constructDefaultMutatingWebhookConfig(caData)
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

func (wrc *Register) createResourceValidatingWebhookConfiguration(caData []byte) error {
	var config *admregapi.ValidatingWebhookConfiguration

	if wrc.serverIP != "" {
		config = wrc.constructDefaultDebugValidatingWebhookConfig(caData)
	} else {
		config = wrc.constructDefaultValidatingWebhookConfig(caData)
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
func (wrc *Register) createPolicyValidatingWebhookConfiguration(caData []byte) error {
	var config *admregapi.ValidatingWebhookConfiguration

	if wrc.serverIP != "" {
		config = wrc.constructDebugPolicyValidatingWebhookConfig(caData)
	} else {
		config = wrc.constructPolicyValidatingWebhookConfig(caData)
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

func (wrc *Register) createPolicyMutatingWebhookConfiguration(caData []byte) error {
	var config *admregapi.MutatingWebhookConfiguration

	if wrc.serverIP != "" {
		config = wrc.constructDebugPolicyMutatingWebhookConfig(caData)
	} else {
		config = wrc.constructPolicyMutatingWebhookConfig(caData)
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

func (wrc *Register) createVerifyMutatingWebhookConfiguration(caData []byte) error {
	var config *admregapi.MutatingWebhookConfiguration

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
	wrc.log.V(3).Info("deleting all webhook configurations")
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

	mutatingConfig := getPolicyMutatingWebhookConfigurationName(wrc.serverIP)

	logger := wrc.log.WithValues("kind", kindMutating, "name", mutatingConfig)

	if mutateCache, ok := wrc.resCache.GetGVRCache("MutatingWebhookConfiguration"); ok {
		if _, err := mutateCache.Lister().Get(mutatingConfig); err != nil && errorsapi.IsNotFound(err) {
			logger.V(4).Info("webhook not found")
			return
		}
	}

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

func getPolicyMutatingWebhookConfigurationName(serverIP string) string {
	var mutatingConfig string
	if serverIP != "" {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationName
	}
	return mutatingConfig
}

func (wrc *Register) removePolicyValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	validatingConfig := getPolicyValidatingWebhookConfigurationName(wrc.serverIP)

	logger := wrc.log.WithValues("kind", kindValidating, "name", validatingConfig)
	if mutateCache, ok := wrc.resCache.GetGVRCache("ValidatingWebhookConfiguration"); ok {
		if _, err := mutateCache.Lister().Get(validatingConfig); err != nil && errorsapi.IsNotFound(err) {
			logger.V(4).Info("webhook not found")
			return
		}
	}

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

func getPolicyValidatingWebhookConfigurationName(serverIP string) string {
	var validatingConfig string
	if serverIP != "" {
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
				admregapi.Rule{
					Resources:   []string{"deployments/*"},
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
				},
				[]admregapi.OperationType{admregapi.Update},
				admregapi.Ignore,
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
				admregapi.Rule{
					Resources:   []string{"deployments/*"},
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
				},
				[]admregapi.OperationType{admregapi.Update},
				admregapi.Ignore,
			),
		},
	}
}

func (wrc *Register) removeVerifyWebhookMutatingWebhookConfig(wg *sync.WaitGroup) {
	defer wg.Done()

	var err error
	mutatingConfig := wrc.getVerifyWebhookMutatingWebhookName()
	logger := wrc.log.WithValues("kind", kindMutating, "name", mutatingConfig)

	if mutateCache, ok := wrc.resCache.GetGVRCache("MutatingWebhookConfiguration"); ok {
		if _, err := mutateCache.Lister().Get(mutatingConfig); err != nil && errorsapi.IsNotFound(err) {
			logger.V(4).Info("webhook not found")
			return
		}
	}

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
			tls.ManagedByLabel: "kyverno",
		},
	}

	secretList, err := wrc.client.ListResource("", "Secret", config.KyvernoNamespace, selector)
	if err != nil {
		wrc.log.Error(err, "failed to clean up Kyverno managed secrets")
		return
	}

	for _, secret := range secretList.Items {
		if err := wrc.client.DeleteResource("", "Secret", secret.GetNamespace(), secret.GetName(), false); err != nil {
			if !errorsapi.IsNotFound(err) {
				wrc.log.Error(err, "failed to delete secret", "ns", secret.GetNamespace(), "name", secret.GetName())
			}
		}
	}
}

func (wrc *Register) checkEndpoint() error {
	obj, err := wrc.client.GetResource("", "Endpoints", config.KyvernoNamespace, config.KyvernoServiceName)
	if err != nil {
		return fmt.Errorf("failed to get endpoint %s/%s: %v", config.KyvernoNamespace, config.KyvernoServiceName, err)
	}
	var endpoint corev1.Endpoints
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &endpoint)
	if err != nil {
		return fmt.Errorf("failed to convert endpoint %s/%s from unstructured: %v", config.KyvernoNamespace, config.KyvernoServiceName, err)
	}

	pods, err := wrc.client.ListResource("", "Pod", config.KyvernoNamespace, &v1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "kyverno"}})
	if err != nil {
		return fmt.Errorf("failed to list Kyverno Pod: %v", err)
	}

	kyverno := pods.Items[0]
	podIP, _, err := unstructured.NestedString(kyverno.UnstructuredContent(), "status", "podIP")
	if err != nil {
		return fmt.Errorf("failed to extract pod IP: %v", err)
	}

	if podIP == "" {
		return fmt.Errorf("pod is not assigned to any node yet")
	}

	for _, subset := range endpoint.Subsets {
		if len(subset.Addresses) == 0 {
			continue
		}

		for _, addr := range subset.Addresses {
			if addr.IP == podIP {
				wrc.log.Info("Endpoint ready", "ns", config.KyvernoNamespace, "name", config.KyvernoServiceName)
				return nil
			}
		}
	}

	// clean up old webhook configurations, if any
	wrc.removeWebhookConfigurations()

	err = fmt.Errorf("endpoint not ready")
	wrc.log.V(3).Info(err.Error(), "ns", config.KyvernoNamespace, "name", config.KyvernoServiceName)
	return err
}

func (wrc *Register) updateResourceValidatingWebhookConfiguration(nsSelector map[string]interface{}) error {
	validatingCache, _ := wrc.resCache.GetGVRCache(kindValidating)

	resourceValidating, err := validatingCache.Lister().Get(getResourceValidatingWebhookConfigName(wrc.serverIP))
	if err != nil {
		return errors.Wrapf(err, "unable to get validatingWebhookConfigurations")
	}

	webhooksUntyped, _, err := unstructured.NestedSlice(resourceValidating.UnstructuredContent(), "webhooks")
	if err != nil {
		return errors.Wrapf(err, "unable to load validatingWebhookConfigurations.webhooks")
	}

	var webhooks map[string]interface{}
	var ok bool
	if webhooksUntyped != nil {
		webhooks, ok = webhooksUntyped[0].(map[string]interface{})
		if !ok {
			return errors.Wrapf(err, "type mismatched, expected map[string]interface{}, got %T", webhooksUntyped[0])
		}
	}
	if err = unstructured.SetNestedMap(webhooks, nsSelector, "namespaceSelector"); err != nil {
		return errors.Wrapf(err, "unable to set validatingWebhookConfigurations.webhooks[0].namespaceSelector")
	}

	if err = unstructured.SetNestedSlice(resourceValidating.UnstructuredContent(), []interface{}{webhooks}, "webhooks"); err != nil {
		return errors.Wrapf(err, "unable to set validatingWebhookConfigurations.webhooks")
	}

	if _, err := wrc.client.UpdateResource(resourceValidating.GetAPIVersion(), resourceValidating.GetKind(), "", resourceValidating, false); err != nil {
		return err
	}

	return nil
}

func (wrc *Register) updateResourceMutatingWebhookConfiguration(nsSelector map[string]interface{}) error {
	mutatingCache, _ := wrc.resCache.GetGVRCache(kindMutating)

	resourceMutating, err := mutatingCache.Lister().Get(getResourceMutatingWebhookConfigName(wrc.serverIP))
	if err != nil {
		return errors.Wrapf(err, "unable to get mutatingWebhookConfigurations")
	}

	webhooksUntyped, _, err := unstructured.NestedSlice(resourceMutating.UnstructuredContent(), "webhooks")
	if err != nil {
		return errors.Wrapf(err, "unable to load mutatingWebhookConfigurations.webhooks")
	}

	var webhooks map[string]interface{}
	var ok bool
	if webhooksUntyped != nil {
		webhooks, ok = webhooksUntyped[0].(map[string]interface{})
		if !ok {
			return errors.Wrapf(err, "type mismatched, expected map[string]interface{}, got %T", webhooksUntyped[0])
		}
	}
	if err = unstructured.SetNestedMap(webhooks, nsSelector, "namespaceSelector"); err != nil {
		return errors.Wrapf(err, "unable to set mutatingWebhookConfigurations.webhooks[0].namespaceSelector")
	}

	if err = unstructured.SetNestedSlice(resourceMutating.UnstructuredContent(), []interface{}{webhooks}, "webhooks"); err != nil {
		return errors.Wrapf(err, "unable to set mutatingWebhookConfigurations.webhooks")
	}

	if _, err := wrc.client.UpdateResource(resourceMutating.GetAPIVersion(), resourceMutating.GetKind(), "", resourceMutating, false); err != nil {
		return err
	}

	return nil
}
