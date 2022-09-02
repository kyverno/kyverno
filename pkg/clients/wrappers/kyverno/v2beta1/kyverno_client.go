package v2beta1

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"k8s.io/client-go/rest"
)

type KyvernoV2beta1Interface interface {
	RESTClient() rest.Interface
	ClusterPoliciesGetter
	PoliciesGetter
}

type KyvernoV2beta1Client struct {
	restClient              rest.Interface
	kyvernov2beta1Interface kyvernov2beta1.KyvernoV2beta1Interface
	clientQueryMetric       utils.ClientQueryMetric
}

func (c *KyvernoV2beta1Client) ClusterPolicies() ClusterPoliciesControlInterface {
	return newClusterPolicies(c)
}

func (c *KyvernoV2beta1Client) Policies(namespace string) PoliciesControlInterface {
	return newPolicies(c, namespace)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *KyvernoV2beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func NewForConfig(restClient rest.Interface, kyvernov2beta1Interface kyvernov2beta1.KyvernoV2beta1Interface, m utils.ClientQueryMetric) *KyvernoV2beta1Client {
	return &KyvernoV2beta1Client{restClient, kyvernov2beta1Interface, m}
}
