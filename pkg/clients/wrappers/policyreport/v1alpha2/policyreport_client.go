package v1alpha2

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

type client struct {
	inner    v1alpha2.Wgpolicyk8sV1alpha2Interface
	recorder metrics.Recorder
}

func (c *client) ClusterPolicyReports() v1alpha2.ClusterPolicyReportInterface {
	return metrics.ClusteredClient[*policyreportv1alpha2.ClusterPolicyReport, *policyreportv1alpha2.ClusterPolicyReportList](
		c.recorder,
		"ClusterPolicyReport",
		metrics.KyvernoClient,
		c.inner.ClusterPolicyReports(),
	)
}

func (c *client) PolicyReports(namespace string) v1alpha2.PolicyReportInterface {
	return metrics.NamespacedClient[*policyreportv1alpha2.PolicyReport, *policyreportv1alpha2.PolicyReportList](
		c.recorder,
		namespace,
		"ClusterPolicyReport",
		metrics.KyvernoClient,
		c.inner.PolicyReports(namespace),
	)
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1alpha2.Wgpolicyk8sV1alpha2Interface, m metrics.Recorder) v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &client{inner, m}
}
