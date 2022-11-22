package client

import (
	priorityclasses "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha1/priorityclasses"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityClass", c.clientType)
	return priorityclasses.WithMetrics(c.inner.PriorityClasses(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.SchedulingV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1alpha1.PriorityClassInterface {
	return priorityclasses.WithTracing(c.inner.PriorityClasses(), c.client, "PriorityClass")
}
