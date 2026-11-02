package client

import (
	"github.com/go-logr/logr"
	compositepodgroups "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha3/compositepodgroups"
	podgroups "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha3/podgroups"
	workloads "github.com/kyverno/kyverno/pkg/clients/kube/schedulingv1alpha3/workloads"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3 "k8s.io/client-go/kubernetes/typed/scheduling/v1alpha3"
	k8sclientgorest "k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface, client string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CompositePodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.CompositePodGroupInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CompositePodGroup", c.clientType)
	return compositepodgroups.WithMetrics(c.inner.CompositePodGroups(namespace), recorder)
}
func (c *withMetrics) PodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.PodGroupInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodGroup", c.clientType)
	return podgroups.WithMetrics(c.inner.PodGroups(namespace), recorder)
}
func (c *withMetrics) Workloads(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.WorkloadInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Workload", c.clientType)
	return workloads.WithMetrics(c.inner.Workloads(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface
	client string
}

func (c *withTracing) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CompositePodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.CompositePodGroupInterface {
	return compositepodgroups.WithTracing(c.inner.CompositePodGroups(namespace), c.client, "CompositePodGroup")
}
func (c *withTracing) PodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.PodGroupInterface {
	return podgroups.WithTracing(c.inner.PodGroups(namespace), c.client, "PodGroup")
}
func (c *withTracing) Workloads(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.WorkloadInterface {
	return workloads.WithTracing(c.inner.Workloads(namespace), c.client, "Workload")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.SchedulingV1alpha3Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CompositePodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.CompositePodGroupInterface {
	return compositepodgroups.WithLogging(c.inner.CompositePodGroups(namespace), c.logger.WithValues("resource", "CompositePodGroups").WithValues("namespace", namespace))
}
func (c *withLogging) PodGroups(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.PodGroupInterface {
	return podgroups.WithLogging(c.inner.PodGroups(namespace), c.logger.WithValues("resource", "PodGroups").WithValues("namespace", namespace))
}
func (c *withLogging) Workloads(namespace string) k8s_io_client_go_kubernetes_typed_scheduling_v1alpha3.WorkloadInterface {
	return workloads.WithLogging(c.inner.Workloads(namespace), c.logger.WithValues("resource", "Workloads").WithValues("namespace", namespace))
}
