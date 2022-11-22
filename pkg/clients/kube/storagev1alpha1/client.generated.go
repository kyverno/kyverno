package client

import (
	"github.com/go-logr/logr"
	csistoragecapacities "github.com/kyverno/kyverno/pkg/clients/kube/storagev1alpha1/csistoragecapacities"
	volumeattachments "github.com/kyverno/kyverno/pkg/clients/kube/storagev1alpha1/volumeattachments"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_storage_v1alpha1 "k8s.io/client-go/kubernetes/typed/storage/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CSIStorageCapacity", c.clientType)
	return csistoragecapacities.WithMetrics(c.inner.CSIStorageCapacities(namespace), recorder)
}
func (c *withMetrics) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "VolumeAttachment", c.clientType)
	return volumeattachments.WithMetrics(c.inner.VolumeAttachments(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	return csistoragecapacities.WithTracing(c.inner.CSIStorageCapacities(namespace), c.client, "CSIStorageCapacity")
}
func (c *withTracing) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	return volumeattachments.WithTracing(c.inner.VolumeAttachments(), c.client, "VolumeAttachment")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_storage_v1alpha1.StorageV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1alpha1.CSIStorageCapacityInterface {
	return csistoragecapacities.WithLogging(c.inner.CSIStorageCapacities(namespace), c.logger.WithValues("resource", "CSIStorageCapacities").WithValues("namespace", namespace))
}
func (c *withLogging) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1alpha1.VolumeAttachmentInterface {
	return volumeattachments.WithLogging(c.inner.VolumeAttachments(), c.logger.WithValues("resource", "VolumeAttachments"))
}
