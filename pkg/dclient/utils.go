package client

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
)

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func newUnstructuredWithSpec(apiVersion, kind, namespace, name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := newUnstructured(apiVersion, kind, namespace, name)
	u.Object["spec"] = spec
	return u
}

func logDiscoveryErrors(err error, c serverPreferredResources) {
	discoveryError := err.(*discovery.ErrGroupDiscoveryFailed)
	for gv, e := range discoveryError.Groups {
		if gv.Group == "custom.metrics.k8s.io" || gv.Group == "metrics.k8s.io" || gv.Group == "external.metrics.k8s.io" {
			// These errors occur when Prometheus is installed as an external metrics server
			// See: https://github.com/kyverno/kyverno/issues/1490
			c.log.V(3).Info("failed to retrieve metrics API group", "gv", gv)
			continue
		}

		c.log.Error(e, "failed to retrieve API group", "gv", gv)
	}
}

func isMetricsServerUnavailable(kind string, err error) bool {
	// error message is defined at:
	// https://github.com/kubernetes/apimachinery/blob/2456ebdaba229616fab2161a615148884b46644b/pkg/api/errors/errors.go#L432
	return strings.HasPrefix(kind, "metrics.k8s.io/") &&
		strings.Contains(err.Error(), "the server is currently unable to handle the request")
}
