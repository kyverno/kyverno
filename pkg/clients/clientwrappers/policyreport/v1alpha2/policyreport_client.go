package v1alpha2

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/clientwrappers/utils"
	"k8s.io/client-go/rest"
)

type Wgpolicyk8sV1alpha2Interface interface {
	RESTClient() rest.Interface
	ClusterPolicyReportsGetter
	PolicyReportsGetter
}

type Wgpolicyk8sV1alpha2Client struct {
	restClient                   rest.Interface
	wgpolicyk8sV1alpha2Interface policyreportv1alpha2.Wgpolicyk8sV1alpha2Interface
	clientQueryMetric            utils.ClientQueryMetric
}

func (c *Wgpolicyk8sV1alpha2Client) ClusterPolicyReports() ClusterPolicyReportControlInterface {
	return newClusterPolicyReports(c)
}

func (c *Wgpolicyk8sV1alpha2Client) PolicyReports(namespace string) PolicyReportControlInterface {
	return newPolicyReports(c, namespace)
}

func (c *Wgpolicyk8sV1alpha2Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func NewForConfig(restClient rest.Interface, wgpolicyk8sV1alpha2Interface policyreportv1alpha2.Wgpolicyk8sV1alpha2Interface, m utils.ClientQueryMetric) *Wgpolicyk8sV1alpha2Client {
	return &Wgpolicyk8sV1alpha2Client{restClient, wgpolicyk8sV1alpha2Interface, m}
}
