package v1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/rest"
)

type client struct {
	inner   v1.KyvernoV1Interface
	metrics metrics.MetricsConfigManager
}

func (c *client) ClusterPolicies() v1.ClusterPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1.ClusterPolicy]
		controllerutils.ListClient[*kyvernov1.ClusterPolicyList]
		controllerutils.StatusClient[*kyvernov1.ClusterPolicy]
	}{
		metrics.ObjectClient[*kyvernov1.ClusterPolicy](recorder, c.inner.ClusterPolicies()),
		metrics.ListClient[*kyvernov1.ClusterPolicyList](recorder, c.inner.ClusterPolicies()),
		metrics.StatusClient[*kyvernov1.ClusterPolicy](recorder, c.inner.ClusterPolicies()),
	}
}

func (c *client) Policies(namespace string) v1.PolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Policy", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1.Policy]
		controllerutils.ListClient[*kyvernov1.PolicyList]
		controllerutils.StatusClient[*kyvernov1.Policy]
	}{
		metrics.ObjectClient[*kyvernov1.Policy](recorder, c.inner.Policies(namespace)),
		metrics.ListClient[*kyvernov1.PolicyList](recorder, c.inner.Policies(namespace)),
		metrics.StatusClient[*kyvernov1.Policy](recorder, c.inner.Policies(namespace)),
	}
}

func (c *client) GenerateRequests(namespace string) v1.GenerateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "GenerateRequest", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1.GenerateRequest]
		controllerutils.ListClient[*kyvernov1.GenerateRequestList]
		controllerutils.StatusClient[*kyvernov1.GenerateRequest]
	}{
		metrics.ObjectClient[*kyvernov1.GenerateRequest](recorder, c.inner.GenerateRequests(namespace)),
		metrics.ListClient[*kyvernov1.GenerateRequestList](recorder, c.inner.GenerateRequests(namespace)),
		metrics.StatusClient[*kyvernov1.GenerateRequest](recorder, c.inner.GenerateRequests(namespace)),
	}
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1.KyvernoV1Interface, metrics metrics.MetricsConfigManager) v1.KyvernoV1Interface {
	return &client{inner, metrics}
}
