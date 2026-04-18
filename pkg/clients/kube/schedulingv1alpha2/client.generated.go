package client

import (
	"github.com/go-logr/logr"
	podgroups "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha2/podgroups"
	workloads "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha2/workloads"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha2"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface, client string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) PodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.PodGroupInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodGroup", c.clientType)
	return podgroups.WithMetrics(c.inner.PodGroups(namespace), recorder)
}
func (c *withMetrics) Workloads(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.WorkloadInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Workload", c.clientType)
	return workloads.WithMetrics(c.inner.Workloads(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) PodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.PodGroupInterface {
	return podgroups.WithTracing(c.inner.PodGroups(namespace), c.client, "PodGroup")
}
func (c *withTracing) Workloads(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.WorkloadInterface {
	return workloads.WithTracing(c.inner.Workloads(namespace), c.client, "Workload")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.SchedulingV1alpha2Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) PodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.PodGroupInterface {
	return podgroups.WithLogging(c.inner.PodGroups(namespace), c.logger.WithValues("resource", "PodGroups").WithValues("namespace", namespace))
}
func (c *withLogging) Workloads(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha2.WorkloadInterface {
	return workloads.WithLogging(c.inner.Workloads(namespace), c.logger.WithValues("resource", "Workloads").WithValues("namespace", namespace))
}
