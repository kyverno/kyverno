package v1alpha2

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
)

type client struct {
	v1alpha2.Wgpolicyk8sV1alpha2Interface
	clientQueryMetric utils.ClientQueryMetric
}

func (c *client) ClusterPolicyReports() v1alpha2.ClusterPolicyReportInterface {
	return wrapClusterPolicyReports(c.Wgpolicyk8sV1alpha2Interface.ClusterPolicyReports(), c.clientQueryMetric)
}

func (c *client) PolicyReports(namespace string) v1alpha2.PolicyReportInterface {
	return wrapPolicyReports(c.Wgpolicyk8sV1alpha2Interface.PolicyReports(namespace), c.clientQueryMetric, namespace)
}

func Wrap(inner v1alpha2.Wgpolicyk8sV1alpha2Interface, m utils.ClientQueryMetric) v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &client{inner, m}
}
