package clientset

import (
	"github.com/go-logr/logr"
	apiextensionsv1 "github.com/kyverno/kyverno/pkg/clients/apiserver/apiextensionsv1"
	apiextensionsv1beta1 "github.com/kyverno/kyverno/pkg/clients/apiserver/apiextensionsv1beta1"
	discovery "github.com/kyverno/kyverno/pkg/clients/apiserver/discovery"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
)

type clientset struct {
	discovery            k8s_io_client_go_discovery.DiscoveryInterface
	apiextensionsv1      k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1.ApiextensionsV1Interface
	apiextensionsv1beta1 k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface
}

func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.discovery
}
func (c *clientset) ApiextensionsV1() k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1.ApiextensionsV1Interface {
	return c.apiextensionsv1
}
func (c *clientset) ApiextensionsV1beta1() k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface {
	return c.apiextensionsv1beta1
}

func WrapWithMetrics(inner k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset.Interface, m metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset.Interface {
	return &clientset{
		discovery:            discovery.WithMetrics(inner.Discovery(), metrics.ClusteredClientQueryRecorder(m, "Discovery", clientType)),
		apiextensionsv1:      apiextensionsv1.WithMetrics(inner.ApiextensionsV1(), m, clientType),
		apiextensionsv1beta1: apiextensionsv1beta1.WithMetrics(inner.ApiextensionsV1beta1(), m, clientType),
	}
}

func WrapWithTracing(inner k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset.Interface) k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset.Interface {
	return &clientset{
		discovery:            discovery.WithTracing(inner.Discovery(), "Discovery", ""),
		apiextensionsv1:      apiextensionsv1.WithTracing(inner.ApiextensionsV1(), "ApiextensionsV1"),
		apiextensionsv1beta1: apiextensionsv1beta1.WithTracing(inner.ApiextensionsV1beta1(), "ApiextensionsV1beta1"),
	}
}

func WrapWithLogging(inner k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset.Interface, logger logr.Logger) k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset.Interface {
	return &clientset{
		discovery:            discovery.WithLogging(inner.Discovery(), logger.WithValues("group", "Discovery")),
		apiextensionsv1:      apiextensionsv1.WithLogging(inner.ApiextensionsV1(), logger.WithValues("group", "ApiextensionsV1")),
		apiextensionsv1beta1: apiextensionsv1beta1.WithLogging(inner.ApiextensionsV1beta1(), logger.WithValues("group", "ApiextensionsV1beta1")),
	}
}
