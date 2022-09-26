package v1beta1

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"k8s.io/client-go/rest"
)

type client struct {
	inner             v1beta1.KyvernoV1beta1Interface
	clientQueryMetric utils.ClientQueryMetric
}

func (c *client) UpdateRequests(namespace string) v1beta1.UpdateRequestInterface {
	return wrapUpdateRequests(c.inner.UpdateRequests(namespace), c.clientQueryMetric, namespace)
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1beta1.KyvernoV1beta1Interface, m utils.ClientQueryMetric) v1beta1.KyvernoV1beta1Interface {
	return &client{inner, m}
}
