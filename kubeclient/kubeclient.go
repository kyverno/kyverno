package kubeclient

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nirmata/kube-policy/config"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	event "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// KubeClient is the api-client for core Kubernetes objects
type KubeClient struct {
	client *kubernetes.Clientset
	logger *log.Logger
}

// Checks parameters and creates new instance of KubeClient
func NewKubeClient(config *rest.Config, logger *log.Logger) (*KubeClient, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "Kubernetes client: ", log.LstdFlags|log.Lshortfile)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeClient{
		logger: logger,
		client: client,
	}, nil
}

func (kc *KubeClient) GetEventsInterface(namespace string) event.EventInterface {
	return kc.client.CoreV1().Events(namespace)
}

func (kc *KubeClient) GetKubePolicyDeployment() (*apps.Deployment, error) {
	kubePolicyDeployment, err := kc.client.
		AppsV1().
		Deployments(config.KubePolicyNamespace).
		Get(config.KubePolicyDeploymentName, meta.GetOptions{})

	if err != nil {
		return nil, err
	}

	return kubePolicyDeployment, nil
}

// Generates new ConfigMap in given namespace. If the namespace does not exists yet,
// waits until it is created for maximum namespaceCreationMaxWaitTime (see below)
func (kc *KubeClient) GenerateConfigMap(generator types.Generation, namespace string) error {
	kc.logger.Printf("Preparing to create configmap %s/%s", namespace, generator.Name)
	configMap := &v1.ConfigMap{}

	var err error

	kc.logger.Printf("Copying data from configmap %s/%s", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
	configMap, err = kc.client.CoreV1().ConfigMaps(generator.CopyFrom.Namespace).Get(generator.CopyFrom.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	configMap.ObjectMeta = metav1.ObjectMeta{
		Name:      generator.Name,
		Namespace: namespace,
	}

	// Copy data from generator to the new configmap
	if generator.Data != nil {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		for k, v := range generator.Data {
			configMap.Data[k] = v
		}
	}

	go kc.createConfigMapAfterNamespaceIsCreated(*configMap, namespace)
	return nil
}

// Generates new Secret in given namespace. If the namespace does not exists yet,
// waits until it is created for maximum namespaceCreationMaxWaitTime (see below)
func (kc *KubeClient) GenerateSecret(generator types.Generation, namespace string) error {
	kc.logger.Printf("Preparing to create secret %s/%s", namespace, generator.Name)
	secret := &v1.Secret{}

	var err error

	kc.logger.Printf("Copying data from secret %s/%s", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
	secret, err = kc.client.CoreV1().Secrets(generator.CopyFrom.Namespace).Get(generator.CopyFrom.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	secret.ObjectMeta = metav1.ObjectMeta{
		Name:      generator.Name,
		Namespace: namespace,
	}

	// Copy data from generator to the new secret
	if generator.Data != nil {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		for k, v := range generator.Data {
			secret.Data[k] = []byte(v)
		}
	}

	go kc.createSecretAfterNamespaceIsCreated(*secret, namespace)
	return nil
}

func defaultDeleteOptions() *metav1.DeleteOptions {
	var deletePeriod int64 = 0
	return &metav1.DeleteOptions{
		GracePeriodSeconds: &deletePeriod,
	}
}

const namespaceCreationMaxWaitTime time.Duration = 30 * time.Second
const namespaceCreationWaitInterval time.Duration = 100 * time.Millisecond

// Waits until namespace is created with maximum duration maxWaitTimeForNamespaceCreation
func (kc *KubeClient) waitUntilNamespaceIsCreated(name string) error {
	timeStart := time.Now()

	var lastError error = nil
	for time.Now().Sub(timeStart) < namespaceCreationMaxWaitTime {
		_, lastError = kc.client.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
		if lastError == nil {
			break
		}
		time.Sleep(namespaceCreationWaitInterval)
	}
	return lastError
}

func (kc *KubeClient) createConfigMapAfterNamespaceIsCreated(configMap v1.ConfigMap, namespace string) {
	err := kc.waitUntilNamespaceIsCreated(namespace)
	if err == nil {
		_, err = kc.client.CoreV1().ConfigMaps(namespace).Create(&configMap)
	}
	if err != nil {
		kc.logger.Printf("Can't create a configmap: %s", err)
	}
}

func (kc *KubeClient) createSecretAfterNamespaceIsCreated(secret v1.Secret, namespace string) {
	err := kc.waitUntilNamespaceIsCreated(namespace)
	if err == nil {
		_, err = kc.client.CoreV1().Secrets(namespace).Create(&secret)
	}
	if err != nil {
		kc.logger.Printf("Can't create a secret: %s", err)
	}
}

var rMapper = map[string]getter{
	"ConfigMap":               configMapGetter,
	"Pods":                    podsGetter,
	"Deployment":              deploymentGetter,
	"CronJob":                 cronJobGetter,
	"Endpoints":               endpointsbGetter,
	"HorizontalPodAutoscaler": horizontalPodAutoscalerGetter,
	"Ingress":                 ingressGetter,
	"Job":                     jobGetter,
	"LimitRange":              limitRangeGetter,
	"Namespace":               namespaceGetter,
	"NetworkPolicy":           networkPolicyGetter,
	"PersistentVolumeClaim":   persistentVolumeClaimGetter,
	"PodDisruptionBudget":     podDisruptionBudgetGetter,
	"PodTemplate":             podTemplateGetter,
	"ResourceQuota":           resourceQuotaGetter,
	"Secret":                  secretGetter,
	"Service":                 serviceGetter,
	"StatefulSet":             statefulSetGetter,
}

var lMapper = map[string]lister{
	"ConfigMap":               configMapLister,
	"Pods":                    podLister,
	"Deployment":              deploymentLister,
	"CronJob":                 cronJobLister,
	"Endpoints":               endpointsLister,
	"HorizontalPodAutoscaler": horizontalPodAutoscalerLister,
	"Ingress":                 ingressLister,
	"Job":                     jobLister,
	"LimitRange":              limitRangeLister,
	"Namespace":               namespaceLister,
	"NetworkPolicy":           networkPolicyLister,
	"PersistentVolumeClaim":   persistentVolumeClaimLister,
	"PodDisruptionBudget":     podDisruptionBudgetLister,
	"PodTemplate":             podTemplateLister,
	"ResourceQuota":           resourceQuotaLister,
	"Secret":                  secretLister,
	"Service":                 serviceLister,
	"StatefulSet":             statefulSetLister,
}

type getter func(*kubernetes.Clientset, string, string) (runtime.Object, error)
type lister func(*kubernetes.Clientset, string) ([]runtime.Object, error)

//ListResource to return resource list
func (kc *KubeClient) ListResource(kind string, namespace string) ([]runtime.Object, error) {
	return lMapper[kind](kc.client, namespace)
}

//GetResource get the resource object
func (kc *KubeClient) GetResource(kind string, resource string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", resource))
		return nil, err
	}
	return rMapper[kind](kc.client, namespace, name)
}

//GetSupportedKinds provides list of supported types
func GetSupportedKinds() (rTypes []string) {
	for k := range rMapper {
		rTypes = append(rTypes, k)
	}
	return rTypes
}

func configMapGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func configMapLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().ConfigMaps(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func podsGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func podLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func deploymentGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
func deploymentLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func cronJobGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.BatchV1beta1().CronJobs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func cronJobLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.BatchV1beta1().CronJobs(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func endpointsbGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().Endpoints(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func endpointsLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().Endpoints(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func horizontalPodAutoscalerGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func horizontalPodAutoscalerLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func ingressGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.ExtensionsV1beta1().Ingresses(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func ingressLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.ExtensionsV1beta1().Ingresses(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func jobGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func jobLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func limitRangeGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().LimitRanges(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
func limitRangeLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().LimitRanges(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func namespaceGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func namespaceLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func networkPolicyGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.NetworkingV1().NetworkPolicies(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func networkPolicyLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.NetworkingV1().NetworkPolicies(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func persistentVolumeClaimGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func persistentVolumeClaimLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func podDisruptionBudgetGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func podDisruptionBudgetLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.PolicyV1beta1().PodDisruptionBudgets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func podTemplateGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().PodTemplates(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func podTemplateLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().PodTemplates(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func resourceQuotaGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().ResourceQuotas(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func resourceQuotaLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().ResourceQuotas(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func secretGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func secretLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func serviceGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func serviceLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.CoreV1().Services(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}

func statefulSetGetter(clientSet *kubernetes.Clientset, namespace string, name string) (runtime.Object, error) {
	obj, err := clientSet.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func statefulSetLister(clientSet *kubernetes.Clientset, namespace string) ([]runtime.Object, error) {
	list, err := clientSet.AppsV1().StatefulSets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objList := []runtime.Object{}
	for _, obj := range list.Items {
		objList = append(objList, &obj)
	}
	return objList, nil
}
