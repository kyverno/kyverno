package kubeclient

import (
	"log"
	"os"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubeClient struct {
	logger *log.Logger
	client *kubernetes.Clientset
}

func NewKubeClient(config *rest.Config, logger *log.Logger) (*KubeClient, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "Policy Controller: ", log.LstdFlags|log.Lshortfile)
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

func (kc *KubeClient) CopySecret(from *types.PolicyCopyFrom, namespaceTo string) error {
	// This is the test code, which works
	var secret v1.Secret
	secret.Namespace = namespaceTo
	secret.ObjectMeta.SetName("test-secret")
	secret.StringData = make(map[string]string)
	secret.StringData["test-data"] = "test-value"
	newSecret, err := kc.client.CoreV1().Secrets(namespaceTo).Create(&secret)
	if err != nil {
		kc.logger.Printf("Unable to create secret: %s", err)
	} else {
		kc.logger.Printf("Secret created: %s", newSecret.Name)
	}
	return err
}
