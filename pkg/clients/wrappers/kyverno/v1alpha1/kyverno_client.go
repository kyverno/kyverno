package v1alpha1

import (
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/rest"
)

type client struct {
	inner   v1alpha1.KyvernoV1alpha1Interface
	metrics metrics.MetricsConfigManager
}

func (c *client) CleanupPolicies(namespace string) v1alpha1.CleanupPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CleanupPolicy", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1alpha1.CleanupPolicy]
		controllerutils.ListClient[*kyvernov1alpha1.CleanupPolicyList]
		controllerutils.StatusClient[*kyvernov1alpha1.CleanupPolicy]
	}{
		metrics.ObjectClient[*kyvernov1alpha1.CleanupPolicy](recorder, c.inner.CleanupPolicies(namespace)),
		metrics.ListClient[*kyvernov1alpha1.CleanupPolicyList](recorder, c.inner.CleanupPolicies(namespace)),
		metrics.StatusClient[*kyvernov1alpha1.CleanupPolicy](recorder, c.inner.CleanupPolicies(namespace)),
	}
}

func (c *client) ClusterCleanupPolicies() v1alpha1.ClusterCleanupPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, "", "ClusterCleanupPolicy", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1alpha1.ClusterCleanupPolicy]
		controllerutils.ListClient[*kyvernov1alpha1.ClusterCleanupPolicyList]
		controllerutils.StatusClient[*kyvernov1alpha1.ClusterCleanupPolicy]
	}{
		metrics.ObjectClient[*kyvernov1alpha1.ClusterCleanupPolicy](recorder, c.inner.ClusterCleanupPolicies()),
		metrics.ListClient[*kyvernov1alpha1.ClusterCleanupPolicyList](recorder, c.inner.ClusterCleanupPolicies()),
		metrics.StatusClient[*kyvernov1alpha1.ClusterCleanupPolicy](recorder, c.inner.ClusterCleanupPolicies()),
	}
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1alpha1.KyvernoV1alpha1Interface, metrics metrics.MetricsConfigManager) v1alpha1.KyvernoV1alpha1Interface {
	return &client{inner, metrics}
}
