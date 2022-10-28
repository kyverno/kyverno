package v1beta1

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/rest"
)

type client struct {
	inner   v1beta1.KyvernoV1beta1Interface
	metrics metrics.MetricsConfigManager
}

func (c *client) UpdateRequests(namespace string) v1beta1.UpdateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "UpdateRequest", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1beta1.UpdateRequest]
		controllerutils.ListClient[*kyvernov1beta1.UpdateRequestList]
		controllerutils.StatusClient[*kyvernov1beta1.UpdateRequest]
	}{
		metrics.ObjectClient[*kyvernov1beta1.UpdateRequest](recorder, c.inner.UpdateRequests(namespace)),
		metrics.ListClient[*kyvernov1beta1.UpdateRequestList](recorder, c.inner.UpdateRequests(namespace)),
		metrics.StatusClient[*kyvernov1beta1.UpdateRequest](recorder, c.inner.UpdateRequests(namespace)),
	}
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1beta1.KyvernoV1beta1Interface, metrics metrics.MetricsConfigManager) v1beta1.KyvernoV1beta1Interface {
	return &client{inner, metrics}
}
