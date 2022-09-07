package v1

import (
	v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
)

type client struct {
	v1.KyvernoV1Interface
	clientQueryMetric utils.ClientQueryMetric
}

func (c *client) ClusterPolicies() v1.ClusterPolicyInterface {
	return wrapClusterPolicies(c.KyvernoV1Interface.ClusterPolicies(), c.clientQueryMetric)
}

func (c *client) Policies(namespace string) v1.PolicyInterface {
	return wrapPolicies(c.KyvernoV1Interface.Policies(namespace), c.clientQueryMetric, namespace)
}

func Wrap(inner v1.KyvernoV1Interface, m utils.ClientQueryMetric) v1.KyvernoV1Interface {
	return &client{inner, m}
}
