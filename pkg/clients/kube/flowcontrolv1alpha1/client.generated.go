package client

import (
	flowschemas "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1alpha1/flowschemas"
	prioritylevelconfigurations "github.com/kyverno/kyverno/pkg/clients/kube/flowcontrolv1alpha1/prioritylevelconfigurations"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1 "k8s.io/client-go/kubernetes/typed/flowcontrol/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "FlowSchema", c.clientType)
	return flowschemas.WithMetrics(c.inner.FlowSchemas(), recorder)
}
func (c *withMetrics) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityLevelConfiguration", c.clientType)
	return prioritylevelconfigurations.WithMetrics(c.inner.PriorityLevelConfigurations(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowcontrolV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) FlowSchemas() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.FlowSchemaInterface {
	return flowschemas.WithTracing(c.inner.FlowSchemas(), c.client, "FlowSchema")
}
func (c *withTracing) PriorityLevelConfigurations() k8s_io_client_go_kubernetes_typed_flowcontrol_v1alpha1.PriorityLevelConfigurationInterface {
	return prioritylevelconfigurations.WithTracing(c.inner.PriorityLevelConfigurations(), c.client, "PriorityLevelConfiguration")
}
