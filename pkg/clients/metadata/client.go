package clientset

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/metadata/resource"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
)

type namespaceableInterface interface {
	Namespace(string) metadata.ResourceInterface
}

func WrapWithMetrics(inner metadata.Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) metadata.Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WrapWithTracing(inner metadata.Interface) metadata.Interface {
	return &withTracing{inner}
}

func WrapWithLogging(inner metadata.Interface, logger logr.Logger) metadata.Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      metadata.Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

type withMetricsNamespaceable struct {
	metrics    metrics.MetricsConfigManager
	resource   string
	clientType metrics.ClientType
	inner      namespaceableInterface
}

func (c *withMetricsNamespaceable) Namespace(namespace string) metadata.ResourceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, c.resource, c.clientType)
	return resource.WithMetrics(c.inner.Namespace(namespace), recorder)
}

func (c *withMetrics) Resource(gvr schema.GroupVersionResource) metadata.Getter {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, gvr.Resource, c.clientType)
	inner := c.inner.Resource(gvr)
	return struct {
		metadata.ResourceInterface
		namespaceableInterface
	}{
		resource.WithMetrics(inner, recorder),
		&withMetricsNamespaceable{c.metrics, gvr.Resource, c.clientType, inner},
	}
}

type withTracing struct {
	inner metadata.Interface
}

type withTracingNamespaceable struct {
	client string
	kind   string
	inner  namespaceableInterface
}

func (c *withTracingNamespaceable) Namespace(namespace string) metadata.ResourceInterface {
	return resource.WithTracing(c.inner.Namespace(namespace), c.client, c.kind)
}

func (c *withTracing) Resource(gvr schema.GroupVersionResource) metadata.Getter {
	inner := c.inner.Resource(gvr)
	client := gvr.GroupResource().String()
	kind := gvr.Resource
	return struct {
		metadata.ResourceInterface
		namespaceableInterface
	}{
		resource.WithTracing(inner, client, kind),
		&withTracingNamespaceable{client, kind, inner},
	}
}

type withLogging struct {
	inner  metadata.Interface
	logger logr.Logger
}

type withLoggingNamespaceable struct {
	inner  namespaceableInterface
	logger logr.Logger
}

func (c *withLoggingNamespaceable) Namespace(namespace string) metadata.ResourceInterface {
	return resource.WithLogging(c.inner.Namespace(namespace), c.logger)
}

func (c *withLogging) Resource(gvr schema.GroupVersionResource) metadata.Getter {
	logger := c.logger.WithValues("group", gvr.Group, "version", gvr.Version, "resource", gvr.Resource)
	inner := c.inner.Resource(gvr)
	return struct {
		metadata.ResourceInterface
		namespaceableInterface
	}{
		resource.WithLogging(inner, logger),
		&withLoggingNamespaceable{inner, logger},
	}
}
