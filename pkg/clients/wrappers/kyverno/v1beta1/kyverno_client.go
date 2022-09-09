package v1beta1

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
)

type client struct {
	v1beta1.KyvernoV1beta1Interface
	clientQueryMetric utils.ClientQueryMetric
}

func (c *client) UpdateRequests(namespace string) v1beta1.UpdateRequestInterface {
	return wrapUpdateRequests(c.KyvernoV1beta1Interface.UpdateRequests(namespace), c.clientQueryMetric, namespace)
}

func Wrap(inner v1beta1.KyvernoV1beta1Interface, m utils.ClientQueryMetric) v1beta1.KyvernoV1beta1Interface {
	return &client{inner, m}
}
