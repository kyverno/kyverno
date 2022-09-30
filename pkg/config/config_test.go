package config_test

import (
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
)

func Test_CreateClientConfig_WithKubeConfig(t *testing.T) {
	cf := createMinimalKubeconfig(t)
	defer os.Remove(cf)
	_, err := config.CreateClientConfig(cf, 0, 0)
	assert.NilError(t, err)
}

func Test_CreateClientConfig_SetBurstQPS(t *testing.T) {
	const (
		qps   = 55
		burst = 99
	)
	cf := createMinimalKubeconfig(t)
	defer os.Remove(cf)
	c, err := config.CreateClientConfig(cf, qps, burst)
	assert.NilError(t, err)
	assert.Equal(t, float32(qps), c.QPS)
	assert.Equal(t, burst, c.Burst)
}

func Test_CreateClientConfig_LimitQPStoFloat32(t *testing.T) {
	qps := float64(math.MaxFloat32) * 2
	cf := createMinimalKubeconfig(t)
	defer os.Remove(cf)
	_, err := config.CreateClientConfig(cf, qps, 0)
	assert.ErrorContains(t, err, "QPS")
}
func createMinimalKubeconfig(t *testing.T) string {
	t.Helper()

	minimalConfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"test": {Server: "http://localhost:7777"},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test": {},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"test": {AuthInfo: "test", Cluster: "test"},
		},
		CurrentContext: "test",
	}

	f, err := os.CreateTemp("", "")
	assert.NilError(t, err)
	enc, err := runtime.Encode(clientcmdlatest.Codec, &minimalConfig)
	assert.NilError(t, err)
	_, err = f.Write(enc)
	assert.NilError(t, err)
	assert.NilError(t, f.Close())

	return f.Name()
}

func createCustomKubeConfig(t *testing.T, fileName string, hosts map[string]string, currentContext string) {
	t.Helper()
	err := os.WriteFile(fileName, []byte(fmt.Sprintf(`
apiVersion: v1
clusters:
- cluster:
    server: %s
  name: dev
- cluster:
    server: %s
  name: qa
contexts:
- context:
    cluster: dev
    user: dev
  name: dev
- context:
    cluster: qa
    user: qa
  name: qa
current-context: %s
kind: Config
preferences: {}
users:
- name: dev
  user: {}
- name: qa
  user: {}

`, hosts["dev"], hosts["qa"], currentContext)), os.FileMode(0755))
	assert.NilError(t, err)
}

func Test_CreateCustomClientConfig_WithContext(t *testing.T) {
	pwd, _ := os.Getwd()
	customKubeConfig := pwd + "/kubeConfig"
	hosts := map[string]string{
		"dev": "http://127.0.0.1:8081",
		"qa":  "http://127.0.0.2:8082",
	}
	currentContext := "dev"
	createCustomKubeConfig(t, customKubeConfig, hosts, currentContext)
	defer os.Remove(customKubeConfig)

	testCases := []struct {
		testName   string
		kubeConfig string
		context    string
		host       string
	}{
		{
			testName:   "default kubeconfig",
			kubeConfig: "",
			context:    "",
		},
		{
			testName:   "custom kubeconfig file with current-context as dev",
			kubeConfig: customKubeConfig,
			context:    "",
			host:       hosts["dev"],
		},
		{
			testName:   "custom kubeconfig file with custom context as qa",
			kubeConfig: customKubeConfig,
			context:    "qa",
			host:       hosts["qa"],
		},
	}

	for _, test := range testCases {
		restConfig, err := config.CreateClientConfigWithContext(test.kubeConfig, test.context)
		assert.NilError(t, err, fmt.Sprintf("test %s failed", test.testName))
		if test.host != "" {
			assert.Equal(t, restConfig.Host, test.host, fmt.Sprintf("test %s failed", test.testName))
		}
	}

	t.Setenv("KUBECONFIG", customKubeConfig) // use custom kubeconfig instead of ~/.kube/config
	newCustomKubeConfig := pwd + "/newkubeConfig"
	newHosts := map[string]string{
		"dev": "http://127.0.0.1:8083",
		"qa":  "http://127.0.0.2:8084",
	}
	createCustomKubeConfig(t, newCustomKubeConfig, newHosts, currentContext)
	defer os.Remove(newCustomKubeConfig)
	testCases = []struct {
		testName   string
		kubeConfig string
		context    string
		host       string
	}{
		{
			testName:   "kubeconfig file from env with current-context as dev",
			kubeConfig: "",
			context:    "",
			host:       hosts["dev"],
		},
		{
			testName:   "kubeconfig file from env with custom context as qa",
			kubeConfig: "",
			context:    "qa",
			host:       hosts["qa"],
		},
		{
			testName:   "override kubeconfig from env with new kubeconfig and custom context as qa",
			kubeConfig: newCustomKubeConfig,
			context:    "qa",
			host:       newHosts["qa"],
		},
	}

	for _, test := range testCases {
		restConfig, err := config.CreateClientConfigWithContext(test.kubeConfig, test.context)
		assert.NilError(t, err, fmt.Sprintf("test %s failed", test.testName))
		if test.host != "" {
			assert.Equal(t, restConfig.Host, test.host, fmt.Sprintf("test %s failed", test.testName))
		}
	}
}
