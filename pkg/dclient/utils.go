package client

import (
	"strings"

	"k8s.io/client-go/discovery"
)

func logDiscoveryErrors(err error, c serverPreferredResources) {
	discoveryError := err.(*discovery.ErrGroupDiscoveryFailed)
	for gv, e := range discoveryError.Groups {
		if gv.Group == "custom.metrics.k8s.io" || gv.Group == "metrics.k8s.io" || gv.Group == "external.metrics.k8s.io" {
			// These errors occur when Prometheus is installed as an external metrics server
			// See: https://github.com/kyverno/kyverno/issues/1490
			logger.V(3).Info("failed to retrieve metrics API group", "gv", gv)
			continue
		}
		logger.Error(e, "failed to retrieve API group", "gv", gv)
	}
}

func isMetricsServerUnavailable(kind string, err error) bool {
	// error message is defined at:
	// https://github.com/kubernetes/apimachinery/blob/2456ebdaba229616fab2161a615148884b46644b/pkg/api/errors/errors.go#L432
	return strings.HasPrefix(kind, "metrics.k8s.io/") &&
		strings.Contains(err.Error(), "the server is currently unable to handle the request")
}
