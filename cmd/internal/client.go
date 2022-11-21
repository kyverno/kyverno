package internal

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	dyn "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kube "github.com/kyverno/kyverno/pkg/clients/kube"
	kyverno "github.com/kyverno/kyverno/pkg/clients/kyverno"
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

func CreateKubernetesClient(logger logr.Logger, opts ...kube.NewOption) kubernetes.Interface {
	logger = logger.WithName("kube-client")
	logger.Info("create kube client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := kube.NewForConfig(CreateClientConfig(logger), opts...)
	checkError(logger, err, "failed to create kubernetes client")
	return client
}

func CreateKyvernoClient(logger logr.Logger, opts ...kyverno.NewOption) versioned.Interface {
	logger = logger.WithName("kyverno-client")
	logger.Info("create kyverno client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := kyverno.NewForConfig(CreateClientConfig(logger), opts...)
	checkError(logger, err, "failed to create kubernetes client")
	return client
}

func CreateDynamicClient(logger logr.Logger, opts ...dyn.NewOption) dynamic.Interface {
	logger = logger.WithName("dynamic-client")
	logger.Info("create dynamic client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := dyn.NewForConfig(CreateClientConfig(logger), opts...)
	checkError(logger, err, "failed to create dynamic client")
	return client
}
