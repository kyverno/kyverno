package v1beta1

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/clientwrappers/utils"
	"k8s.io/client-go/rest"
)

type KyvernoV1beta1Interface interface {
	RESTClient() rest.Interface
	UpdateRequestsGetter
}

type KyvernoV1beta1Client struct {
	restClient              rest.Interface
	kyvernov1beta1Interface kyvernov1beta1.KyvernoV1beta1Interface
	clientQueryMetric       utils.ClientQueryMetric
}

func (c *KyvernoV1beta1Client) UpdateRequests(namespace string) UpdateRequestControlInterface {
	return newUpdateRequests(c, namespace)
}

func (c *KyvernoV1beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func NewForConfig(restClient rest.Interface, kyvernov1beta1Interface kyvernov1beta1.KyvernoV1beta1Interface, m utils.ClientQueryMetric) *KyvernoV1beta1Client {
	return &KyvernoV1beta1Client{restClient, kyvernov1beta1Interface, m}
}
