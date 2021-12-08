package config_test

import (
	"math"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"

	"github.com/kyverno/kyverno/pkg/config"
)

func Test_CreateClientConfig_WithKubeConfig(t *testing.T) {
	cf := createMinimalKubeconfig(t)
	defer os.Remove(cf)
	_, err := config.CreateClientConfig(cf, 0, 0, logr.Discard())
	assert.NilError(t, err)
}

func Test_CreateClientConfig_SetBurstQPS(t *testing.T) {
	const (
		qps   = 55
		burst = 99
	)

	cf := createMinimalKubeconfig(t)
	defer os.Remove(cf)
	c, err := config.CreateClientConfig(cf, qps, burst, logr.Discard())
	assert.NilError(t, err)
	assert.Equal(t, float32(qps), c.QPS)
	assert.Equal(t, burst, c.Burst)
}

func Test_CreateClientConfig_LimitQPStoFloat32(t *testing.T) {
	qps := float64(math.MaxFloat32) * 2

	cf := createMinimalKubeconfig(t)
	defer os.Remove(cf)
	_, err := config.CreateClientConfig(cf, qps, 0, logr.Discard())
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
