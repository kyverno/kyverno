package client

import (
	horizontalpodautoscalers "github.com/kyverno/kyverno/pkg/clients/kube/autoscalingv2/horizontalpodautoscalers"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_autoscaling_v2 "k8s.io/client-go/kubernetes/typed/autoscaling/v2"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface, client string) k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) HorizontalPodAutoscalers(namespace string) k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "HorizontalPodAutoscaler", c.clientType)
	return horizontalpodautoscalers.WithMetrics(c.inner.HorizontalPodAutoscalers(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_autoscaling_v2.AutoscalingV2Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) HorizontalPodAutoscalers(namespace string) k8s_io_client_go_kubernetes_typed_autoscaling_v2.HorizontalPodAutoscalerInterface {
	return horizontalpodautoscalers.WithTracing(c.inner.HorizontalPodAutoscalers(namespace), c.client, "HorizontalPodAutoscaler")
}
