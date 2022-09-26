package v1alpha2

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"k8s.io/client-go/rest"
)

type client struct {
	inner             v1alpha2.KyvernoV1alpha2Interface
	clientQueryMetric utils.ClientQueryMetric
}

func (c *client) ClusterReportChangeRequests() v1alpha2.ClusterReportChangeRequestInterface {
	return wrapClusterReportChangeRequests(c.inner.ClusterReportChangeRequests(), c.clientQueryMetric)
}

func (c *client) ReportChangeRequests(namespace string) v1alpha2.ReportChangeRequestInterface {
	return wrapReportChangeRequests(c.inner.ReportChangeRequests(namespace), c.clientQueryMetric, namespace)
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1alpha2.KyvernoV1alpha2Interface, m utils.ClientQueryMetric) v1alpha2.KyvernoV1alpha2Interface {
	return &client{inner, m}
}
