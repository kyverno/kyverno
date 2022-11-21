package internal

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func CreateClientConfig(logger logr.Logger) *rest.Config {
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst)
	checkError(logger, err, "failed to create rest client configuration")
	return clientConfig
}

func CreateKubernetesClient(logger logr.Logger) kubernetes.Interface {
	logger = logger.WithName("kube-client")
	logger.Info("create kube client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := kubernetes.NewForConfig(CreateClientConfig(logger))
	checkError(logger, err, "failed to create kubernetes client")
	return client
}

func CreateKyvernoClient(logger logr.Logger) versioned.Interface {
	logger = logger.WithName("kyverno-client")
	logger.Info("create kyverno client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := versioned.NewForConfig(CreateClientConfig(logger))
	checkError(logger, err, "failed to create kubernetes client")
	return client
}

func CreateDynamicClient(logger logr.Logger) dynamic.Interface {
	logger = logger.WithName("dynamic-client")
	logger.Info("create dynamic client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := dynamic.NewForConfig(CreateClientConfig(logger))
	checkError(logger, err, "failed to create dynamic client")
	return client
}
