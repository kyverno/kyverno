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

func (c *client) ClusterAdmissionReports() v1alpha2.ClusterAdmissionReportInterface {
	return wrapClusterAdmissionReports(c.inner.ClusterAdmissionReports(), c.clientQueryMetric)
}

func (c *client) ClusterBackgroundScanReports() v1alpha2.ClusterBackgroundScanReportInterface {
	return wrapClusterBackgroundScanReports(c.inner.ClusterBackgroundScanReports(), c.clientQueryMetric)
}

func (c *client) AdmissionReports(namespace string) v1alpha2.AdmissionReportInterface {
	return wrapAdmissionReports(c.inner.AdmissionReports(namespace), c.clientQueryMetric, namespace)
}

func (c *client) BackgroundScanReports(namespace string) v1alpha2.BackgroundScanReportInterface {
	return wrapBackgroundScanReports(c.inner.BackgroundScanReports(namespace), c.clientQueryMetric, namespace)
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1alpha2.KyvernoV1alpha2Interface, m utils.ClientQueryMetric) v1alpha2.KyvernoV1alpha2Interface {
	return &client{inner, m}
}
