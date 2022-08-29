package v1

import (
	kyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"k8s.io/client-go/rest"
)

type KyvernoV1Interface interface {
	RESTClient() rest.Interface
	ClusterPoliciesGetter
	PoliciesGetter
}

type KyvernoV1Client struct {
	restClient         rest.Interface
	kyvernov1Interface kyvernov1.KyvernoV1Interface
	clientQueryMetric  utils.ClientQueryMetric
}

func (c *KyvernoV1Client) ClusterPolicies() ClusterPoliciesControlInterface {
	return newClusterPolicies(c)
}

func (c *KyvernoV1Client) Policies(namespace string) PoliciesControlInterface {
	return newPolicies(c, namespace)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *KyvernoV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func NewForConfig(restClient rest.Interface, kyvernov1Interface kyvernov1.KyvernoV1Interface, m utils.ClientQueryMetric) *KyvernoV1Client {
	return &KyvernoV1Client{restClient, kyvernov1Interface, m}
}
