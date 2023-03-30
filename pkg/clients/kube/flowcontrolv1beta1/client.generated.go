package client

import (
	"github.com/go-logr/logr"
	flowschemas "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1beta1/flowschemas"
	prioritylevelconfigurations "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1beta1/prioritylevelconfigurations"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "FlowSchema", c.clientType)
	return flowschemas.WithMetrics(c.inner.FlowSchemas(), recorder)
}
func (c *withMetrics) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityLevelConfiguration", c.clientType)
	return prioritylevelconfigurations.WithMetrics(c.inner.PriorityLevelConfigurations(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	return flowschemas.WithTracing(c.inner.FlowSchemas(), c.client, "FlowSchema")
}
func (c *withTracing) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	return prioritylevelconfigurations.WithTracing(c.inner.PriorityLevelConfigurations(), c.client, "PriorityLevelConfiguration")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowcontrolV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.FlowSchemaInterface {
	return flowschemas.WithLogging(c.inner.FlowSchemas(), c.logger.WithValues("resource", "FlowSchemas"))
}
func (c *withLogging) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1beta1.PriorityLevelConfigurationInterface {
	return prioritylevelconfigurations.WithLogging(c.inner.PriorityLevelConfigurations(), c.logger.WithValues("resource", "PriorityLevelConfigurations"))
}
