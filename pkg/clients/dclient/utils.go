package dclient

import (
	"strings"

	"k8s.io/client-go/discovery"
)

func logDiscoveryErrors(err error) {
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

// isServerCurrentlyUnableToHandleRequest returns true if the error is related to the discovery not able to handle the request
// this can happen with aggregated services when the api server can't get a `TokenReview` and is not able to send requests to
// the underlying service, this is typically due to kyverno blocking `TokenReview` admission requests.
func isServerCurrentlyUnableToHandleRequest(err error) bool {
	return err != nil && strings.Contains(err.Error(), "the server is currently unable to handle the request")
}
