package client

import (
	"github.com/go-logr/logr"
	csidrivers "github.com/kyverno/kyverno/pkg/clients/kube/storagev1beta1/csidrivers"
	csinodes "github.com/kyverno/kyverno/pkg/clients/kube/storagev1beta1/csinodes"
	csistoragecapacities "github.com/kyverno/kyverno/pkg/clients/kube/storagev1beta1/csistoragecapacities"
	storageclasses "github.com/kyverno/kyverno/pkg/clients/kube/storagev1beta1/storageclasses"
	volumeattachments "github.com/kyverno/kyverno/pkg/clients/kube/storagev1beta1/volumeattachments"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_storage_v1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CSIDriver", c.clientType)
	return csidrivers.WithMetrics(c.inner.CSIDrivers(), recorder)
}
func (c *withMetrics) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CSINode", c.clientType)
	return csinodes.WithMetrics(c.inner.CSINodes(), recorder)
}
func (c *withMetrics) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CSIStorageCapacity", c.clientType)
	return csistoragecapacities.WithMetrics(c.inner.CSIStorageCapacities(namespace), recorder)
}
func (c *withMetrics) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "StorageClass", c.clientType)
	return storageclasses.WithMetrics(c.inner.StorageClasses(), recorder)
}
func (c *withMetrics) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "VolumeAttachment", c.clientType)
	return volumeattachments.WithMetrics(c.inner.VolumeAttachments(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	return csidrivers.WithTracing(c.inner.CSIDrivers(), c.client, "CSIDriver")
}
func (c *withTracing) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	return csinodes.WithTracing(c.inner.CSINodes(), c.client, "CSINode")
}
func (c *withTracing) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	return csistoragecapacities.WithTracing(c.inner.CSIStorageCapacities(namespace), c.client, "CSIStorageCapacity")
}
func (c *withTracing) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	return storageclasses.WithTracing(c.inner.StorageClasses(), c.client, "StorageClass")
}
func (c *withTracing) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	return volumeattachments.WithTracing(c.inner.VolumeAttachments(), c.client, "VolumeAttachment")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CSIDrivers() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIDriverInterface {
	return csidrivers.WithLogging(c.inner.CSIDrivers(), c.logger.WithValues("resource", "CSIDrivers"))
}
func (c *withLogging) CSINodes() k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSINodeInterface {
	return csinodes.WithLogging(c.inner.CSINodes(), c.logger.WithValues("resource", "CSINodes"))
}
func (c *withLogging) CSIStorageCapacities(namespace string) k8s_io_client_go_kubernetes_typed_storage_v1beta1.CSIStorageCapacityInterface {
	return csistoragecapacities.WithLogging(c.inner.CSIStorageCapacities(namespace), c.logger.WithValues("resource", "CSIStorageCapacities").WithValues("namespace", namespace))
}
func (c *withLogging) StorageClasses() k8s_io_client_go_kubernetes_typed_storage_v1beta1.StorageClassInterface {
	return storageclasses.WithLogging(c.inner.StorageClasses(), c.logger.WithValues("resource", "StorageClasses"))
}
func (c *withLogging) VolumeAttachments() k8s_io_client_go_kubernetes_typed_storage_v1beta1.VolumeAttachmentInterface {
	return volumeattachments.WithLogging(c.inner.VolumeAttachments(), c.logger.WithValues("resource", "VolumeAttachments"))
}
