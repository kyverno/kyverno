package v1beta1

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/rest"
)

type client struct {
	inner    v1beta1.KyvernoV1beta1Interface
	recorder metrics.Recorder
}

func (c *client) UpdateRequests(namespace string) v1beta1.UpdateRequestInterface {
	return struct {
		controllerutils.Client[*kyvernov1beta1.UpdateRequest, *kyvernov1beta1.UpdateRequestList]
		controllerutils.StatusClient[*kyvernov1beta1.UpdateRequest]
	}{
		metrics.NamespacedClient[*kyvernov1beta1.UpdateRequest, *kyvernov1beta1.UpdateRequestList](
			c.recorder,
			namespace,
			"UpdateRequest",
			metrics.KyvernoClient,
			c.inner.UpdateRequests(namespace),
		),
		metrics.NamespacedStatusClient[*kyvernov1beta1.UpdateRequest](
			c.recorder,
			namespace,
			"UpdateRequest",
			metrics.KyvernoClient,
			c.inner.UpdateRequests(namespace),
		),
	}
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1beta1.KyvernoV1beta1Interface, m metrics.Recorder) v1beta1.KyvernoV1beta1Interface {
	return &client{inner, m}
}
