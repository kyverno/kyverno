package client

import (
	cronjobs "github.com/kyverno/kyverno/pkg/clients/kube/batchv1/cronjobs"
	jobs "github.com/kyverno/kyverno/pkg/clients/kube/batchv1/jobs"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_batch_v1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface, client string) k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CronJob", c.clientType)
	return cronjobs.WithMetrics(c.inner.CronJobs(namespace), recorder)
}
func (c *withMetrics) Jobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Job", c.clientType)
	return jobs.WithMetrics(c.inner.Jobs(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_batch_v1.BatchV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1.CronJobInterface {
	return cronjobs.WithTracing(c.inner.CronJobs(namespace), c.client, "CronJob")
}
func (c *withTracing) Jobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1.JobInterface {
	return jobs.WithTracing(c.inner.Jobs(namespace), c.client, "Job")
}
