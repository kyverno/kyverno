package client

import (
	"github.com/go-logr/logr"
	apiservices "github.com/kyverno/kyverno/pkg/clients/aggregator/apiregistrationv1/apiservices"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
	k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
)

func WithMetrics(inner k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface, client string) k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface, logger logr.Logger) k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) APIServices() k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.APIServiceInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "APIService", c.clientType)
	return apiservices.WithMetrics(c.inner.APIServices(), recorder)
}

type withTracing struct {
	inner  k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) APIServices() k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.APIServiceInterface {
	return apiservices.WithTracing(c.inner.APIServices(), c.client, "APIService")
}

type withLogging struct {
	inner  k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.ApiregistrationV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) APIServices() k8s_io_kube_aggregator_pkg_client_clientset_generated_clientset_typed_apiregistration_v1.APIServiceInterface {
	return apiservices.WithLogging(c.inner.APIServices(), c.logger.WithValues("resource", "APIServices"))
}
