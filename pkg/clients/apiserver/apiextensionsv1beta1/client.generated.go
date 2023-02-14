package client

import (
	"github.com/go-logr/logr"
	customresourcedefinitions "github.com/kyverno/kyverno/pkg/clients/apiserver/apiextensionsv1beta1/customresourcedefinitions"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface, client string) k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface, logger logr.Logger) k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CustomResourceDefinitions() k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.CustomResourceDefinitionInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CustomResourceDefinition", c.clientType)
	return customresourcedefinitions.WithMetrics(c.inner.CustomResourceDefinitions(), recorder)
}

type withTracing struct {
	inner  k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CustomResourceDefinitions() k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.CustomResourceDefinitionInterface {
	return customresourcedefinitions.WithTracing(c.inner.CustomResourceDefinitions(), c.client, "CustomResourceDefinition")
}

type withLogging struct {
	inner  k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.ApiextensionsV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CustomResourceDefinitions() k8s_io_apiextensions_apiserver_pkg_client_clientset_clientset_typed_apiextensions_v1beta1.CustomResourceDefinitionInterface {
	return customresourcedefinitions.WithLogging(c.inner.CustomResourceDefinitions(), c.logger.WithValues("resource", "CustomResourceDefinitions"))
}
