package config_test

import (
	"math"
	"testing"

	"gotest.tools/assert"
	"k8s.io/client-go/rest"

	"github.com/kyverno/kyverno/pkg/config"
)

func Test_CreateClientConfig_WithKubeConfig(t *testing.T) {
	c := &rest.Config{}
	err := config.ConfigureClientConfig(c, 0, 0)
	assert.NilError(t, err)
}

func Test_CreateClientConfig_SetBurstQPS(t *testing.T) {
	const (
		qps   = 55
		burst = 99
	)
	c := &rest.Config{}
	err := config.ConfigureClientConfig(c, qps, burst)
	assert.NilError(t, err)
	assert.Equal(t, float32(qps), c.QPS)
	assert.Equal(t, burst, c.Burst)
}

func Test_CreateClientConfig_LimitQPStoFloat32(t *testing.T) {
	qps := float64(math.MaxFloat32) * 2
	c := &rest.Config{}
	err := config.ConfigureClientConfig(c, qps, 0)
	assert.ErrorContains(t, err, "QPS")
}
