package webhookconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
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
	// clients
	kubeClient    kubernetes.Interface
	kyvernoClient versioned.Interface
	clientConfig  *rest.Config

	// listers
	mwcLister   admissionregistrationv1listers.MutatingWebhookConfigurationLister
	vwcLister   admissionregistrationv1listers.ValidatingWebhookConfigurationLister
	kDeplLister appsv1listers.DeploymentLister

	metricsConfig metrics.MetricsConfigManager

	// channels
	stopCh               <-chan struct{}
	UpdateWebhookChan    chan bool
	createDefaultWebhook chan string

	serverIP           string // when running outside a cluster
	timeoutSeconds     int32
	log                logr.Logger
	debug              bool
	autoUpdateWebhooks bool

	// manage implements methods to manage webhook configurations
	manage
}

// NewRegister creates new Register instance
func NewRegister(
	clientConfig *rest.Config,
	client dclient.Interface,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	kDeplInformer appsv1informers.DeploymentInformer,
	pInformer kyvernov1informers.ClusterPolicyInformer,
	npInformer kyvernov1informers.PolicyInformer,
	metricsConfig metrics.MetricsConfigManager,
	serverIP string,
	webhookTimeout int32,
	debug bool,
	autoUpdateWebhooks bool,
	stopCh <-chan struct{},
	log logr.Logger,
) *Register {
	register := &Register{
		clientConfig:         clientConfig,
		kubeClient:           kubeClient,
		kyvernoClient:        kyvernoClient,
		mwcLister:            mwcInformer.Lister(),
		vwcLister:            vwcInformer.Lister(),
		kDeplLister:          kDeplInformer.Lister(),
		metricsConfig:        metricsConfig,
		UpdateWebhookChan:    make(chan bool),
		createDefaultWebhook: make(chan string),
		stopCh:               stopCh,
		serverIP:             serverIP,
		timeoutSeconds:       webhookTimeout,
		log:                  log.WithName("Register"),
		debug:                debug,
		autoUpdateWebhooks:   autoUpdateWebhooks,
	}

	register.manage = newWebhookConfigManager(client.Discovery(), kubeClient, kyvernoClient, pInformer, npInformer, mwcInformer, vwcInformer, metricsConfig, serverIP, register.autoUpdateWebhooks, register.createDefaultWebhook, stopCh, log.WithName("WebhookConfigManager"))

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
	caData := wrc.readCaData()
	if caData == nil {
		return errors.New("Unable to extract CA data from configuration")
	}
	var errors []string
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
	if _, err := wrc.mwcLister.Get(getVerifyMutatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}
	if _, err := wrc.mwcLister.Get(getResourceMutatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}
	if _, err := wrc.vwcLister.Get(getResourceValidatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}
	if _, err := wrc.mwcLister.Get(getPolicyMutatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}
	if _, err := wrc.vwcLister.Get(getPolicyValidatingWebhookConfigName(wrc.serverIP)); err != nil {
		return err
	}
	return nil
}

// Remove removes all webhook configurations
func (wrc *Register) Remove(cleanupKyvernoResource bool, wg *sync.WaitGroup) {
	defer wg.Done()
	// delete Lease object to let init container do the cleanup
	if err := wrc.kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()).Delete(context.TODO(), "kyvernopre-lock", metav1.DeleteOptions{}); err != nil && errorsapi.IsNotFound(err) {
		wrc.metricsConfig.RecordClientQueries(metrics.ClientDelete, metrics.KubeClient, "Lease", config.KyvernoNamespace())
		wrc.log.WithName("cleanup").Error(err, "failed to clean up Lease lock")
	}
	if cleanupKyvernoResource {
		wrc.removeWebhookConfigurations()
	}
}

func (wrc *Register) ResetPolicyStatus(kyvernoInTermination bool, wg *sync.WaitGroup) {
	defer wg.Done()

	if !kyvernoInTermination {
		return
	}

	logger := wrc.log.WithName("ResetPolicyStatus")
	cpols, err := wrc.kyvernoClient.KyvernoV1().ClusterPolicies().List(context.TODO(), metav1.ListOptions{})
	if err == nil {
		for _, item := range cpols.Items {
			cpol := item
			cpol.Status.SetReady(false)
			if _, err := wrc.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), &cpol, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "failed to set ClusterPolicy status READY=false", "name", cpol.GetName())
			}
		}
	} else {
		logger.Error(err, "failed to list clusterpolicies")
	}

	pols, err := wrc.kyvernoClient.KyvernoV1().Policies(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err == nil {
		for _, item := range pols.Items {
			pol := item
			pol.Status.SetReady(false)
			if _, err := wrc.kyvernoClient.KyvernoV1().Policies(pol.GetNamespace()).UpdateStatus(context.TODO(), &pol, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "failed to set Policy status READY=false", "namespace", pol.GetNamespace(), "name", pol.GetName())
			}
		}
	} else {
		logger.Error(err, "failed to list namespaced policies")
	}
}

// GetWebhookTimeOut returns the value of webhook timeout
func (wrc *Register) GetWebhookTimeOut() time.Duration {
	return time.Duration(wrc.timeoutSeconds)
}

func (wrc *Register) UpdateWebhooksCaBundle() error {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			managedByLabel: kyvernov1.ValueKyvernoApp,
		},
	}
	caData := wrc.readCaData()
	m := wrc.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations()
	v := wrc.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations()

	wrc.metricsConfig.RecordClientQueries(metrics.ClientList, metrics.KubeClient, kindMutating, "")
	if list, err := m.List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)}); err != nil {
		return err
	} else {
		for _, item := range list.Items {
			copy := item
			for r := range copy.Webhooks {
				copy.Webhooks[r].ClientConfig.CABundle = caData
			}
			if _, err := m.Update(context.TODO(), &copy, metav1.UpdateOptions{}); err != nil {
				wrc.metricsConfig.RecordClientQueries(metrics.ClientUpdate, metrics.KubeClient, kindMutating, "")
				return err
			}
		}
	}

	wrc.metricsConfig.RecordClientQueries(metrics.ClientList, metrics.KubeClient, kindValidating, "")
	if list, err := v.List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)}); err != nil {
		return err
	} else {
		for _, item := range list.Items {
			copy := item
			for r := range copy.Webhooks {
				copy.Webhooks[r].ClientConfig.CABundle = caData
			}

			wrc.metricsConfig.RecordClientQueries(metrics.ClientUpdate, metrics.KubeClient, kindValidating, "")
			if _, err := v.Update(context.TODO(), &copy, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// UpdateWebhookConfigurations updates resource webhook configurations dynamically
// based on the UPDATEs of Kyverno ConfigMap defined in INIT_CONFIG env
//
// it currently updates namespaceSelector only, can be extend to update other fields
// +deprecated
func (wrc *Register) UpdateWebhookConfigurations(configHandler config.Configuration) {
	logger := wrc.log.WithName("UpdateWebhookConfigurations")
	for {
		<-wrc.UpdateWebhookChan
		logger.V(4).Info("received the signal to update webhook configurations")

		webhookCfgs := configHandler.GetWebhooks()
		webhookCfg := config.WebhookConfig{}
		if len(webhookCfgs) > 0 {
			webhookCfg = webhookCfgs[0]
		}

		retry := false
		if err := wrc.updateResourceMutatingWebhookConfiguration(webhookCfg); err != nil {
			logger.Error(err, "unable to update mutatingWebhookConfigurations", "name", getResourceMutatingWebhookConfigName(wrc.serverIP))
			retry = true
		}

		if err := wrc.updateResourceValidatingWebhookConfiguration(webhookCfg); err != nil {
			logger.Error(err, "unable to update validatingWebhookConfigurations", "name", getResourceValidatingWebhookConfigName(wrc.serverIP))
			retry = true
		}

		if retry {
			go func() {
				time.Sleep(1 * time.Second)
				select {
				case wrc.UpdateWebhookChan <- true:
					return
				default:
					return
				}
			}()
		}
	}
}

func (wrc *Register) ValidateWebhookConfigurations(namespace, name string) error {
	logger := wrc.log.WithName("ValidateWebhookConfigurations")
	cm, err := wrc.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	wrc.metricsConfig.RecordClientQueries(metrics.ClientGet, metrics.KubeClient, "ConfigMap", namespace)
	if err != nil {
		logger.Error(err, "unable to fetch ConfigMap", "namespace", namespace, "name", name)
		return nil
	}
	webhooks, ok := cm.Data["webhooks"]
	if !ok {
		logger.V(4).Info("webhook configurations not defined")
		return nil
	}
	webhookCfgs := make([]config.WebhookConfig, 0, 10)
	return json.Unmarshal([]byte(webhooks), &webhookCfgs)
}

func (wrc *Register) createMutatingWebhookConfiguration(config *admissionregistrationv1.MutatingWebhookConfiguration) error {
	logger := wrc.log.WithValues("kind", kindMutating, "name", config.Name)

	wrc.metricsConfig.RecordClientQueries(metrics.ClientCreate, metrics.KubeClient, kindMutating, "")
	if _, err := wrc.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), config, metav1.CreateOptions{}); err != nil {
		if errorsapi.IsAlreadyExists(err) {
			logger.V(6).Info("resource mutating webhook configuration already exists", "name", config.Name)
			return wrc.updateMutatingWebhookConfiguration(config)
		}
		logger.Error(err, "failed to create resource mutating webhook configuration", "name", config.Name)
		return err
	}
	logger.Info("created webhook")
	return nil
}

func (wrc *Register) createValidatingWebhookConfiguration(config *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	logger := wrc.log.WithValues("kind", kindValidating, "name", config.Name)

	wrc.metricsConfig.RecordClientQueries(metrics.ClientCreate, metrics.KubeClient, kindValidating, "")
	if _, err := wrc.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), config, metav1.CreateOptions{}); err != nil {
		if errorsapi.IsAlreadyExists(err) {
			logger.V(6).Info("resource validating webhook configuration already exists", "name", config.Name)
			return wrc.updateValidatingWebhookConfiguration(config)
		}
		logger.Error(err, "failed to create resource validating webhook configuration", "name", config.Name)
		return err
	}
	logger.Info("created webhook")
	return nil
}

func (wrc *Register) createResourceMutatingWebhookConfiguration(caData []byte) error {
	owner := wrc.constructOwner()
	var config *admissionregistrationv1.MutatingWebhookConfiguration
	if wrc.serverIP != "" {
		config = constructDefaultDebugMutatingWebhookConfig(wrc.serverIP, caData, wrc.timeoutSeconds, wrc.autoUpdateWebhooks, owner)
	} else {
		config = constructDefaultMutatingWebhookConfig(caData, wrc.timeoutSeconds, wrc.autoUpdateWebhooks, owner)
	}
	return wrc.createMutatingWebhookConfiguration(config)
}

func (wrc *Register) createResourceValidatingWebhookConfiguration(caData []byte) error {
	owner := wrc.constructOwner()
	var config *admissionregistrationv1.ValidatingWebhookConfiguration
	if wrc.serverIP != "" {
		config = constructDefaultDebugValidatingWebhookConfig(wrc.serverIP, caData, wrc.timeoutSeconds, wrc.autoUpdateWebhooks, owner)
	} else {
		config = constructDefaultValidatingWebhookConfig(caData, wrc.timeoutSeconds, wrc.autoUpdateWebhooks, owner)
	}
	return wrc.createValidatingWebhookConfiguration(config)
}

func (wrc *Register) createPolicyValidatingWebhookConfiguration(caData []byte) error {
	owner := wrc.constructOwner()
	var config *admissionregistrationv1.ValidatingWebhookConfiguration
	if wrc.serverIP != "" {
		config = constructDebugPolicyValidatingWebhookConfig(wrc.serverIP, caData, wrc.timeoutSeconds, owner)
	} else {
		config = constructPolicyValidatingWebhookConfig(caData, wrc.timeoutSeconds, owner)
	}
	return wrc.createValidatingWebhookConfiguration(config)
}

func (wrc *Register) createPolicyMutatingWebhookConfiguration(caData []byte) error {
	owner := wrc.constructOwner()
	var config *admissionregistrationv1.MutatingWebhookConfiguration
	if wrc.serverIP != "" {
		config = constructDebugPolicyMutatingWebhookConfig(wrc.serverIP, caData, wrc.timeoutSeconds, owner)
	} else {
		config = constructPolicyMutatingWebhookConfig(caData, wrc.timeoutSeconds, owner)
	}
	return wrc.createMutatingWebhookConfiguration(config)
}

func (wrc *Register) createVerifyMutatingWebhookConfiguration(caData []byte) error {
	owner := wrc.constructOwner()
	var config *admissionregistrationv1.MutatingWebhookConfiguration
	if wrc.serverIP != "" {
		config = constructDebugVerifyMutatingWebhookConfig(wrc.serverIP, caData, wrc.timeoutSeconds, owner)
	} else {
		config = constructVerifyMutatingWebhookConfig(caData, wrc.timeoutSeconds, owner)
	}
	return wrc.createMutatingWebhookConfiguration(config)
}

func (wrc *Register) checkEndpoint() error {
	endpoint, err := wrc.kubeClient.CoreV1().Endpoints(config.KyvernoNamespace()).Get(context.TODO(), config.KyvernoServiceName(), metav1.GetOptions{})
	wrc.metricsConfig.RecordClientQueries(metrics.ClientGet, metrics.KubeClient, "EndPoint", config.KyvernoNamespace())
	if err != nil {
		return fmt.Errorf("failed to get endpoint %s/%s: %v", config.KyvernoNamespace(), config.KyvernoServiceName(), err)
	}
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
		},
	}
	pods, err := wrc.kubeClient.CoreV1().Pods(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)})
	wrc.metricsConfig.RecordClientQueries(metrics.ClientList, metrics.KubeClient, "Pod", config.KyvernoNamespace())
	if err != nil {
		return fmt.Errorf("failed to list Kyverno Pod: %v", err)
	}
	ips := getHealthyPodsIP(pods.Items)
	if len(ips) == 0 {
		return fmt.Errorf("pod is not assigned to any node yet")
	}
	for _, subset := range endpoint.Subsets {
		if len(subset.Addresses) == 0 {
			continue
		}
		for _, addr := range subset.Addresses {
			if utils.ContainsString(ips, addr.IP) {
				wrc.log.V(2).Info("Endpoint ready", "ns", config.KyvernoNamespace(), "name", config.KyvernoServiceName())
				return nil
			}
		}
	}
	err = fmt.Errorf("endpoint not ready")
	wrc.log.V(3).Info(err.Error(), "ns", config.KyvernoNamespace(), "name", config.KyvernoServiceName())
	return err
}

func (wrc *Register) updateResourceValidatingWebhookConfiguration(webhookCfg config.WebhookConfig) error {
	resource, err := wrc.vwcLister.Get(getResourceValidatingWebhookConfigName(wrc.serverIP))
	if err != nil {
		return errors.Wrapf(err, "unable to get validatingWebhookConfigurations")
	}
	copy := resource.DeepCopy()
	for i := range copy.Webhooks {
		copy.Webhooks[i].ObjectSelector = webhookCfg.ObjectSelector
		copy.Webhooks[i].NamespaceSelector = webhookCfg.NamespaceSelector
	}
	if reflect.DeepEqual(resource.Webhooks, copy.Webhooks) {
		wrc.log.V(4).Info("namespaceSelector unchanged, skip updating validatingWebhookConfigurations")
		return nil
	}
	wrc.metricsConfig.RecordClientQueries(metrics.ClientUpdate, metrics.KubeClient, kindValidating, "")
	if _, err := wrc.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.TODO(), copy, metav1.UpdateOptions{}); err != nil {
		return err
	}
	wrc.log.V(3).Info("successfully updated validatingWebhookConfigurations", "name", getResourceMutatingWebhookConfigName(wrc.serverIP))
	return nil
}

func (wrc *Register) updateResourceMutatingWebhookConfiguration(webhookCfg config.WebhookConfig) error {
	resource, err := wrc.mwcLister.Get(getResourceMutatingWebhookConfigName(wrc.serverIP))
	if err != nil {
		return errors.Wrapf(err, "unable to get mutatingWebhookConfigurations")
	}
	copy := resource.DeepCopy()
	for i := range copy.Webhooks {
		copy.Webhooks[i].ObjectSelector = webhookCfg.ObjectSelector
		copy.Webhooks[i].NamespaceSelector = webhookCfg.NamespaceSelector
	}
	if reflect.DeepEqual(resource.Webhooks, copy.Webhooks) {
		wrc.log.V(4).Info("namespaceSelector unchanged, skip updating mutatingWebhookConfigurations")
		return nil
	}

	wrc.metricsConfig.RecordClientQueries(metrics.ClientUpdate, metrics.KubeClient, kindMutating, "")
	if _, err := wrc.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), copy, metav1.UpdateOptions{}); err != nil {
		return err
	}
	wrc.log.V(3).Info("successfully updated mutatingWebhookConfigurations", "name", getResourceMutatingWebhookConfigName(wrc.serverIP))
	return nil
}

// updateMutatingWebhookConfiguration updates an existing MutatingWebhookConfiguration with the rules provided by
// the targetConfig. If the targetConfig doesn't provide any rules, the existing rules will be preserved.
func (wrc *Register) updateMutatingWebhookConfiguration(targetConfig *admissionregistrationv1.MutatingWebhookConfiguration) error {
	// Fetch the existing webhook.
	currentConfiguration, err := wrc.mwcLister.Get(targetConfig.Name)
	if err != nil {
		return fmt.Errorf("failed to get %s %s: %v", kindMutating, targetConfig.Name, err)
	}
	// Create a map of the target webhooks.
	targetWebhooksMap := make(map[string]admissionregistrationv1.MutatingWebhook)
	for _, w := range targetConfig.Webhooks {
		targetWebhooksMap[w.Name] = w
	}
	// Update the webhooks.
	newWebhooks := make([]admissionregistrationv1.MutatingWebhook, 0)
	for _, w := range currentConfiguration.Webhooks {
		target, exist := targetWebhooksMap[w.Name]
		if !exist {
			continue
		}
		delete(targetWebhooksMap, w.Name)
		// Update the webhook configuration
		w.ClientConfig.URL = target.ClientConfig.URL
		w.ClientConfig.Service = target.ClientConfig.Service
		w.ClientConfig.CABundle = target.ClientConfig.CABundle
		if target.Rules != nil {
			// If the target webhook has rule definitions override the current.
			w.Rules = target.Rules
		}
		newWebhooks = append(newWebhooks, w)
	}
	// Check if there are additional webhooks defined and add them.
	for _, w := range targetWebhooksMap {
		newWebhooks = append(newWebhooks, w)
	}
	// Update the current configuration.
	currentConfiguration.Webhooks = newWebhooks
	wrc.metricsConfig.RecordClientQueries(metrics.ClientUpdate, metrics.KubeClient, kindMutating, "")
	if _, err := wrc.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), currentConfiguration, metav1.UpdateOptions{}); err != nil {
		return err
	}
	wrc.log.V(3).Info("successfully updated mutatingWebhookConfigurations", "name", targetConfig.Name)
	return nil
}

// updateValidatingWebhookConfiguration updates an existing ValidatingWebhookConfiguration with the rules provided by
// the targetConfig. If the targetConfig doesn't provide any rules, the existing rules will be preserved.
func (wrc *Register) updateValidatingWebhookConfiguration(targetConfig *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	// Fetch the existing webhook.
	currentConfiguration, err := wrc.vwcLister.Get(targetConfig.Name)
	if err != nil {
		return fmt.Errorf("failed to get %s %s: %v", kindValidating, targetConfig.Name, err)
	}
	// Create a map of the target webhooks.
	targetWebhooksMap := make(map[string]admissionregistrationv1.ValidatingWebhook)
	for _, w := range targetConfig.Webhooks {
		targetWebhooksMap[w.Name] = w
	}
	// Update the webhooks.
	newWebhooks := make([]admissionregistrationv1.ValidatingWebhook, 0)
	for _, w := range currentConfiguration.Webhooks {
		target, exist := targetWebhooksMap[w.Name]
		if !exist {
			continue
		}
		delete(targetWebhooksMap, w.Name)
		// Update the webhook configuration
		w.ClientConfig.URL = target.ClientConfig.URL
		w.ClientConfig.Service = target.ClientConfig.Service
		w.ClientConfig.CABundle = target.ClientConfig.CABundle
		if target.Rules != nil {
			// If the target webhook has rule definitions override the current.
			w.Rules = target.Rules
		}
		newWebhooks = append(newWebhooks, w)
	}
	// Check if there are additional webhooks defined and add them.
	for _, w := range targetWebhooksMap {
		newWebhooks = append(newWebhooks, w)
	}
	// Update the current configuration.
	currentConfiguration.Webhooks = newWebhooks
	wrc.metricsConfig.RecordClientQueries(metrics.ClientUpdate, metrics.KubeClient, kindValidating, "")
	if _, err := wrc.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.TODO(), currentConfiguration, metav1.UpdateOptions{}); err != nil {
		return err
	}
	wrc.log.V(3).Info("successfully updated validatingWebhookConfigurations", "name", targetConfig.Name)
	return nil
}

func (wrc *Register) ShouldCleanupKyvernoResource() bool {
	logger := wrc.log.WithName("cleanupKyvernoResource")
	deploy, err := wrc.kubeClient.AppsV1().Deployments(config.KyvernoNamespace()).Get(context.TODO(), config.KyvernoDeploymentName(), metav1.GetOptions{})
	wrc.metricsConfig.RecordClientQueries(metrics.ClientGet, metrics.KubeClient, "Deployment", config.KyvernoNamespace())
	if err != nil {
		if errorsapi.IsNotFound(err) {
			logger.Info("Kyverno deployment not found, cleanup Kyverno resources")
			return true
		}
		logger.Error(err, "failed to get deployment, not cleaning up kyverno resources")
		return false
	}
	if deploy.GetDeletionTimestamp() != nil {
		logger.Info("Kyverno is terminating, cleanup Kyverno resources")
		return true
	}
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
		logger.Info("Kyverno is scaled to zero, cleanup Kyverno resources")
		return true
	}
	logger.Info("updating Kyverno Pod, won't clean up Kyverno resources")
	return false
}

func (wrc *Register) removeWebhookConfigurations() {
	startTime := time.Now()
	wrc.log.V(3).Info("deleting all webhook configurations")
	defer wrc.log.V(4).Info("removed webhook configurations", "processingTime", time.Since(startTime).String())
	var wg sync.WaitGroup
	wg.Add(5)
	go wrc.removeResourceMutatingWebhookConfiguration(&wg)
	go wrc.removeResourceValidatingWebhookConfiguration(&wg)
	go wrc.removePolicyMutatingWebhookConfiguration(&wg)
	go wrc.removePolicyValidatingWebhookConfiguration(&wg)
	go wrc.removeVerifyWebhookMutatingWebhookConfig(&wg)
	wg.Wait()
}

func (wrc *Register) removeResourceMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	wrc.removeMutatingWebhookConfiguration(getResourceMutatingWebhookConfigName(wrc.serverIP))
}

func (wrc *Register) removeResourceValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	wrc.removeValidatingWebhookConfiguration(getResourceValidatingWebhookConfigName(wrc.serverIP))
}

func (wrc *Register) removePolicyMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	wrc.removeMutatingWebhookConfiguration(getPolicyMutatingWebhookConfigName(wrc.serverIP))
}

func (wrc *Register) removePolicyValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()
	wrc.removeValidatingWebhookConfiguration(getPolicyValidatingWebhookConfigName(wrc.serverIP))
}

func (wrc *Register) removeVerifyWebhookMutatingWebhookConfig(wg *sync.WaitGroup) {
	defer wg.Done()
	wrc.removeMutatingWebhookConfiguration(getVerifyMutatingWebhookConfigName(wrc.serverIP))
}

func (wrc *Register) removeMutatingWebhookConfiguration(name string) {
	logger := wrc.log.WithValues("kind", kindMutating, "name", name)
	if err := wrc.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil && !errorsapi.IsNotFound(err) {
		logger.Error(err, "failed to delete the mutating webhook configuration")
	} else {
		logger.Info("webhook configuration deleted")
	}
	wrc.metricsConfig.RecordClientQueries(metrics.ClientDelete, metrics.KubeClient, kindMutating, "")
}

func (wrc *Register) removeValidatingWebhookConfiguration(name string) {
	logger := wrc.log.WithValues("kind", kindValidating, "name", name)
	if err := wrc.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil && !errorsapi.IsNotFound(err) {
		logger.Error(err, "failed to delete the validating webhook configuration")
	} else {
		logger.Info("webhook configuration deleted")
	}
	wrc.metricsConfig.RecordClientQueries(metrics.ClientDelete, metrics.KubeClient, kindValidating, "")
}
