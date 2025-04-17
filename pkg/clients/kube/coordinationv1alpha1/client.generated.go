package client

import (
	"github.com/go-logr/logr"
	leasecandidates "github.com/kyverno/kyverno/pkg/clients/kube/coordinationv1alpha1/leasecandidates"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_coordination_v1alpha1 "k8s.io/client-go/kubernetes/typed/coordination/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) LeaseCandidates(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.LeaseCandidateInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "LeaseCandidate", c.clientType)
	return leasecandidates.WithMetrics(c.inner.LeaseCandidates(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) LeaseCandidates(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.LeaseCandidateInterface {
	return leasecandidates.WithTracing(c.inner.LeaseCandidates(namespace), c.client, "LeaseCandidate")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.CoordinationV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) LeaseCandidates(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1alpha1.LeaseCandidateInterface {
	return leasecandidates.WithLogging(c.inner.LeaseCandidates(namespace), c.logger.WithValues("resource", "LeaseCandidates").WithValues("namespace", namespace))
}
