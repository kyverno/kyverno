package v1alpha2

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/rest"
)

type client struct {
	inner   v1alpha2.Wgpolicyk8sV1alpha2Interface
	metrics metrics.MetricsConfigManager
}

func (c *client) ClusterPolicyReports() v1alpha2.ClusterPolicyReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicyReport", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*policyreportv1alpha2.ClusterPolicyReport]
		controllerutils.ListClient[*policyreportv1alpha2.ClusterPolicyReportList]
	}{
		metrics.ObjectClient[*policyreportv1alpha2.ClusterPolicyReport](recorder, c.inner.ClusterPolicyReports()),
		metrics.ListClient[*policyreportv1alpha2.ClusterPolicyReportList](recorder, c.inner.ClusterPolicyReports()),
	}
}

func (c *client) PolicyReports(namespace string) v1alpha2.PolicyReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyReport", metrics.KyvernoClient)
	return struct {
		controllerutils.ObjectClient[*policyreportv1alpha2.PolicyReport]
		controllerutils.ListClient[*policyreportv1alpha2.PolicyReportList]
	}{
		metrics.ObjectClient[*policyreportv1alpha2.PolicyReport](recorder, c.inner.PolicyReports(namespace)),
		metrics.ListClient[*policyreportv1alpha2.PolicyReportList](recorder, c.inner.PolicyReports(namespace)),
	}
}

func (c *client) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}

func Wrap(inner v1alpha2.Wgpolicyk8sV1alpha2Interface, metrics metrics.MetricsConfigManager) v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &client{inner, metrics}
}
