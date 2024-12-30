package config

import (
	"fmt"
	"math"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

// CreateClientConfig creates client config and applies rate limit QPS and burst
func CreateClientConfig(kubeconfig string, qps float64, burst int) (*rest.Config, error) {
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	if qps > math.MaxFloat32 {
		return nil, fmt.Errorf("client rate limit QPS must not be higher than %e", math.MaxFloat32)
	}
	clientConfig.Burst = burst
	clientConfig.QPS = float32(qps)
	return clientConfig, nil
}

// createClientConfig creates client config
func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// CreateClientConfigWithContext creates client config from custom kubeconfig file and context
// Used for cli commands
func CreateClientConfigWithContext(kubeconfig string, context string) (*rest.Config, error) {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	kubernetesConfig.KubeConfig = &kubeconfig
	kubernetesConfig.Context = &context
	return kubernetesConfig.ToRESTConfig()
}
