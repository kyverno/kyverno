package v1alpha2

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/rest"
)

type client struct {
	inner   v1alpha2.KyvernoV1alpha2Interface
	metrics metrics.MetricsConfigManager
}

func (c *client) ClusterAdmissionReports() v1alpha2.ClusterAdmissionReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterAdmissionReport", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1alpha2.ClusterAdmissionReport]
		controllerutils.ListClient[*kyvernov1alpha2.ClusterAdmissionReportList]
	}{
		metrics.ObjectClient[*kyvernov1alpha2.ClusterAdmissionReport](recorder, c.inner.ClusterAdmissionReports()),
		metrics.ListClient[*kyvernov1alpha2.ClusterAdmissionReportList](recorder, c.inner.ClusterAdmissionReports()),
	}
}

func (c *client) ClusterBackgroundScanReports() v1alpha2.ClusterBackgroundScanReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterBackgroundScanReport", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1alpha2.ClusterBackgroundScanReport]
		controllerutils.ListClient[*kyvernov1alpha2.ClusterBackgroundScanReportList]
	}{
		metrics.ObjectClient[*kyvernov1alpha2.ClusterBackgroundScanReport](recorder, c.inner.ClusterBackgroundScanReports()),
		metrics.ListClient[*kyvernov1alpha2.ClusterBackgroundScanReportList](recorder, c.inner.ClusterBackgroundScanReports()),
	}
}

func (c *client) AdmissionReports(namespace string) v1alpha2.AdmissionReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "AdmissionReport", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1alpha2.AdmissionReport]
		controllerutils.ListClient[*kyvernov1alpha2.AdmissionReportList]
	}{
		metrics.ObjectClient[*kyvernov1alpha2.AdmissionReport](recorder, c.inner.AdmissionReports(namespace)),
		metrics.ListClient[*kyvernov1alpha2.AdmissionReportList](recorder, c.inner.AdmissionReports(namespace)),
	}
}

func (c *client) BackgroundScanReports(namespace string) v1alpha2.BackgroundScanReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "BackgroundScanReport", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*kyvernov1alpha2.BackgroundScanReport]
		controllerutils.ListClient[*kyvernov1alpha2.BackgroundScanReportList]
	}{
		metrics.ObjectClient[*kyvernov1alpha2.BackgroundScanReport](recorder, c.inner.BackgroundScanReports(namespace)),
		metrics.ListClient[*kyvernov1alpha2.BackgroundScanReportList](recorder, c.inner.BackgroundScanReports(namespace)),
	}
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1alpha2.KyvernoV1alpha2Interface, metrics metrics.MetricsConfigManager) v1alpha2.KyvernoV1alpha2Interface {
	return &client{inner, metrics}
}
