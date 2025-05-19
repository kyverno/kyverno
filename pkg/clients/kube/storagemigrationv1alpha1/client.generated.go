package client

import (
	"github.com/go-logr/logr"
	storageversionmigrations "github.com/kyverno/kyverno/pkg/clients/kube/storagemigrationv1alpha1/storageversionmigrations"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1 "k8s.io/client-go/kubernetes/typed/storagemigration/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) StorageVersionMigrations() k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StorageVersionMigrationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "StorageVersionMigration", c.clientType)
	return storageversionmigrations.WithMetrics(c.inner.StorageVersionMigrations(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) StorageVersionMigrations() k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StorageVersionMigrationInterface {
	return storageversionmigrations.WithTracing(c.inner.StorageVersionMigrations(), c.client, "StorageVersionMigration")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StoragemigrationV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) StorageVersionMigrations() k8s_io_client_go_kubernetes_typed_storagemigration_v1alpha1.StorageVersionMigrationInterface {
	return storageversionmigrations.WithLogging(c.inner.StorageVersionMigrations(), c.logger.WithValues("resource", "StorageVersionMigrations"))
}
