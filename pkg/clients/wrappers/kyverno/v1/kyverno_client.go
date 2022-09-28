package v1

import (
	v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

type client struct {
	inner             v1.KyvernoV1Interface
	clientQueryMetric metrics.Recorder
}

func (c *client) ClusterPolicies() v1.ClusterPolicyInterface {
	return wrapClusterPolicies(c.inner.ClusterPolicies(), c.clientQueryMetric)
}

func (c *client) Policies(namespace string) v1.PolicyInterface {
	return wrapPolicies(c.inner.Policies(namespace), c.clientQueryMetric, namespace)
}

func (c *client) GenerateRequests(namespace string) v1.GenerateRequestInterface {
	return wrapGenerateRequests(c.inner.GenerateRequests(namespace), c.clientQueryMetric, namespace)
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1.KyvernoV1Interface, m metrics.Recorder) v1.KyvernoV1Interface {
	return &client{inner, m}
}
