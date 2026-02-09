package client

import (
	"github.com/go-logr/logr"
	devicetaintrules "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3/devicetaintrules"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_resource_v1alpha3 "k8s.io/client-go/kubernetes/typed/resource/v1alpha3"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface, client string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) DeviceTaintRules() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.DeviceTaintRuleInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "DeviceTaintRule", c.clientType)
	return devicetaintrules.WithMetrics(c.inner.DeviceTaintRules(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) DeviceTaintRules() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.DeviceTaintRuleInterface {
	return devicetaintrules.WithTracing(c.inner.DeviceTaintRules(), c.client, "DeviceTaintRule")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) DeviceTaintRules() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.DeviceTaintRuleInterface {
	return devicetaintrules.WithLogging(c.inner.DeviceTaintRules(), c.logger.WithValues("resource", "DeviceTaintRules"))
}
