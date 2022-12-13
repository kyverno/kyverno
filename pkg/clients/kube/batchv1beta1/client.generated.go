package client

import (
	"github.com/go-logr/logr"
	cronjobs "github.com/kyverno/kyverno/pkg/clients/kube/batchv1beta1/cronjobs"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_batch_v1beta1 "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CronJob", c.clientType)
	return cronjobs.WithMetrics(c.inner.CronJobs(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	return cronjobs.WithTracing(c.inner.CronJobs(namespace), c.client, "CronJob")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_batch_v1beta1.BatchV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CronJobs(namespace string) k8s_io_client_go_kubernetes_typed_batch_v1beta1.CronJobInterface {
	return cronjobs.WithLogging(c.inner.CronJobs(namespace), c.logger.WithValues("resource", "CronJobs").WithValues("namespace", namespace))
}
