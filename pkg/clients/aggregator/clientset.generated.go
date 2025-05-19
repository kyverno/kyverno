package clientset

import (
	"github.com/go-logr/logr"
	apiregistrationv1 "github.com/kyverno/kyverno/pkg/clients/aggregator/apiregistrationv1"
	apiregistrationv1beta1 "github.com/kyverno/kyverno/pkg/clients/aggregator/apiregistrationv1beta1"
	discovery "github.com/kyverno/kyverno/pkg/clients/aggregator/discovery"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
)

type clientset struct {
	discovery              k8s_io_client_go_discovery.DiscoveryInterface
	apiregistrationv1      k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface
	apiregistrationv1beta1 k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1beta1.ApiregistrationV1beta1Interface
}

func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.discovery
}
func (c *clientset) ApiregistrationV1() k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface {
	return c.apiregistrationv1
}
func (c *clientset) ApiregistrationV1beta1() k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1beta1.ApiregistrationV1beta1Interface {
	return c.apiregistrationv1beta1
}

func WrapWithMetrics(inner k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset.Interface, m metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset.Interface {
	return &clientset{
		discovery:              discovery.WithMetrics(inner.Discovery(), metrics.ClusteredClientQueryRecorder(m, "Discovery", clientType)),
		apiregistrationv1:      apiregistrationv1.WithMetrics(inner.ApiregistrationV1(), m, clientType),
		apiregistrationv1beta1: apiregistrationv1beta1.WithMetrics(inner.ApiregistrationV1beta1(), m, clientType),
	}
}

func WrapWithTracing(inner k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset.Interface) k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset.Interface {
	return &clientset{
		discovery:              discovery.WithTracing(inner.Discovery(), "Discovery", ""),
		apiregistrationv1:      apiregistrationv1.WithTracing(inner.ApiregistrationV1(), "ApiregistrationV1"),
		apiregistrationv1beta1: apiregistrationv1beta1.WithTracing(inner.ApiregistrationV1beta1(), "ApiregistrationV1beta1"),
	}
}

func WrapWithLogging(inner k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset.Interface, logger logr.Logger) k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset.Interface {
	return &clientset{
		discovery:              discovery.WithLogging(inner.Discovery(), logger.WithValues("group", "Discovery")),
		apiregistrationv1:      apiregistrationv1.WithLogging(inner.ApiregistrationV1(), logger.WithValues("group", "ApiregistrationV1")),
		apiregistrationv1beta1: apiregistrationv1beta1.WithLogging(inner.ApiregistrationV1beta1(), logger.WithValues("group", "ApiregistrationV1beta1")),
	}
}
