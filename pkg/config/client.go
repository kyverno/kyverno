package config

import (
	"fmt"
	"math"

	rest "k8s.io/client-go/rest"
)

// ConfigureClientConfig creates client config and applies rate limit QPS and burst
func ConfigureClientConfig(clientConfig *rest.Config, qps float64, burst int) error {
	if qps > math.MaxFloat32 {
		return fmt.Errorf("client rate limit QPS must not be higher than %e", math.MaxFloat32)
	}
	clientConfig.Burst = burst
	clientConfig.QPS = float32(qps)
	return nil
}
