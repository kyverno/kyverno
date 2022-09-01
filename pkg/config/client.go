package config

import (
	"fmt"
	"math"

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
		logger.Info("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	logger.V(4).Info("Using specified kubeconfig", "kubeconfig", kubeconfig)
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
