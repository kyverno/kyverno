package kubeclient

import (
	"log"
	"os"
	"time"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubeClient is the api-client for core Kubernetes objects
type KubeClient struct {
	logger *log.Logger
	client *kubernetes.Clientset
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

// Generates new ConfigMap in given namespace. If the namespace does not exists yet,
// waits until it is created for maximum namespaceCreationMaxWaitTime (see below)
func (kc *KubeClient) GenerateConfigMap(generator types.PolicyConfigGenerator, namespace string) error {
	kc.logger.Printf("Preparing to create configmap %s/%s", namespace, generator.Name)
	configMap := &v1.ConfigMap{}

	var err error
	if generator.CopyFrom != nil {
		kc.logger.Printf("Copying data from configmap %s/%s", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
		configMap, err = kc.client.CoreV1().ConfigMaps(generator.CopyFrom.Namespace).Get(generator.CopyFrom.Name, defaultGetOptions())
		if err != nil {
			return err
		}
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
func (kc *KubeClient) GenerateSecret(generator types.PolicyConfigGenerator, namespace string) error {
	kc.logger.Printf("Preparing to create secret %s/%s", namespace, generator.Name)
	secret := &v1.Secret{}

	var err error
	if generator.CopyFrom != nil {
		kc.logger.Printf("Copying data from secret %s/%s", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
		secret, err = kc.client.CoreV1().Secrets(generator.CopyFrom.Namespace).Get(generator.CopyFrom.Name, defaultGetOptions())
		if err != nil {
			return err
		}
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

func defaultGetOptions() metav1.GetOptions {
	return metav1.GetOptions{
		ResourceVersion:      "1",
		IncludeUninitialized: true,
	}
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
		_, lastError = kc.client.CoreV1().Namespaces().Get(name, defaultGetOptions())
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
