package clientset

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dynamic/resource"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type namespaceableInterface interface {
	Namespace(string) dynamic.ResourceInterface
}

func WrapWithMetrics(inner dynamic.Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) dynamic.Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WrapWithTracing(inner dynamic.Interface) dynamic.Interface {
	return &withTracing{inner}
}

func WrapWithLogging(inner dynamic.Interface, logger logr.Logger) dynamic.Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      dynamic.Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

type withMetricsNamespaceable struct {
	inner      namespaceableInterface
	metrics    metrics.MetricsConfigManager
	resource   string
	clientType metrics.ClientType
}

func (c *withMetricsNamespaceable) Namespace(namespace string) dynamic.ResourceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, c.resource, c.clientType)
	return resource.WithMetrics(c.inner.Namespace(namespace), recorder)
}

func (c *withMetrics) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, gvr.Resource, c.clientType)
	inner := c.inner.Resource(gvr)
	return struct {
		dynamic.ResourceInterface
		namespaceableInterface
	}{
		resource.WithMetrics(inner, recorder),
		&withMetricsNamespaceable{inner, c.metrics, gvr.Resource, c.clientType},
	}
}

type withTracing struct {
	inner dynamic.Interface
}

type withTracingNamespaceable struct {
	inner  namespaceableInterface
	client string
	kind   string
}

func (c *withTracingNamespaceable) Namespace(namespace string) dynamic.ResourceInterface {
	return resource.WithTracing(c.inner.Namespace(namespace), c.client, c.kind)
}

func (c *withTracing) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	inner := c.inner.Resource(gvr)
	client := gvr.GroupResource().String()
	kind := gvr.Resource
	return struct {
		dynamic.ResourceInterface
		namespaceableInterface
	}{
		resource.WithTracing(inner, client, kind),
		&withTracingNamespaceable{inner, client, kind},
	}
}

type withLogging struct {
	inner  dynamic.Interface
	logger logr.Logger
}

type withLoggingNamespaceable struct {
	inner  namespaceableInterface
	logger logr.Logger
}

func (c *withLoggingNamespaceable) Namespace(namespace string) dynamic.ResourceInterface {
	return resource.WithLogging(c.inner.Namespace(namespace), c.logger)
}

func (c *withLogging) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	logger := c.logger.WithValues("group", gvr.Group, "version", gvr.Version, "resource", gvr.Resource)
	inner := c.inner.Resource(gvr)
	return struct {
		dynamic.ResourceInterface
		namespaceableInterface
	}{
		resource.WithLogging(inner, logger),
		&withLoggingNamespaceable{inner, logger},
	}
}
