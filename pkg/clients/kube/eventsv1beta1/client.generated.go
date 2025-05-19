package client

import (
	"github.com/go-logr/logr"
	events "github.com/kyverno/kyverno/pkg/clients/kube/eventsv1beta1/events"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_events_v1beta1 "k8s.io/client-go/kubernetes/typed/events/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) Events(namespace string) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Event", c.clientType)
	return events.WithMetrics(c.inner.Events(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) Events(namespace string) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	return events.WithTracing(c.inner.Events(namespace), c.client, "Event")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_events_v1beta1.EventsV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) Events(namespace string) k8s_io_client_go_kubernetes_typed_events_v1beta1.EventInterface {
	return events.WithLogging(c.inner.Events(namespace), c.logger.WithValues("resource", "Events").WithValues("namespace", namespace))
}
