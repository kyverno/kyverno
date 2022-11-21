package clientset

import (
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

type withMetrics struct {
	inner      dynamic.Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

type withMetricsNamespaceable struct {
	metrics    metrics.MetricsConfigManager
	resource   string
	clientType metrics.ClientType
	inner      namespaceableInterface
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
		&withMetricsNamespaceable{c.metrics, gvr.Resource, c.clientType, inner},
	}
}

type withTracing struct {
	inner dynamic.Interface
}

type withTracingNamespaceable struct {
	client string
	kind   string
	inner  namespaceableInterface
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
		&withTracingNamespaceable{client, kind, inner},
	}
}
