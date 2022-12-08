package client

import (
	"github.com/go-logr/logr"
	priorityclasses "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1beta1/priorityclasses"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_scheduling_v1beta1 "k8s.io/client-go/kubernetes/typed/scheduling/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PriorityClass", c.clientType)
	return priorityclasses.WithMetrics(c.inner.PriorityClasses(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	return priorityclasses.WithTracing(c.inner.PriorityClasses(), c.client, "PriorityClass")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.SchedulingV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) PriorityClasses() k8s_io_client_go_kubernetes_typed_scheduling_v1beta1.PriorityClassInterface {
	return priorityclasses.WithLogging(c.inner.PriorityClasses(), c.logger.WithValues("resource", "PriorityClasses"))
}
