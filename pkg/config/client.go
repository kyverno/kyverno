package config

import (
	"fmt"
	"math"
	"runtime"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/release-utils/version"
)

// withUserAgent explicitly sets the UserAgent field in rest.Config which is
// then used by client-go for calls to the API server, helps with debugging issues
func withUserAgent(restConfig *rest.Config, err error) (*rest.Config, error) {
	if err != nil && restConfig != nil {
		restConfig.UserAgent = fmt.Sprintf("Kyverno/%s (%s; %s)", version.GetVersionInfo().GitVersion, runtime.GOOS, runtime.GOARCH)
	}
	return restConfig, err
}

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
	return withUserAgent(clientConfig, nil)
}

// createClientConfig creates client config
func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		return withUserAgent(rest.InClusterConfig())
	}
	return withUserAgent(clientcmd.BuildConfigFromFlags("", kubeconfig))
}

// CreateClientConfigWithContext creates client config from custom kubeconfig file and context
// Used for cli commands
func CreateClientConfigWithContext(kubeconfig string, context string) (*rest.Config, error) {
	kubernetesConfig := genericclioptions.NewConfigFlags(true)
	kubernetesConfig.KubeConfig = &kubeconfig
	kubernetesConfig.Context = &context
	return withUserAgent(kubernetesConfig.ToRESTConfig())
}
