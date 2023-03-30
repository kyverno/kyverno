package client

import (
	"github.com/go-logr/logr"
	podschedulings "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha1/podschedulings"
	resourceclaims "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha1/resourceclaims"
	resourceclaimtemplates "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha1/resourceclaimtemplates"
	resourceclasses "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha1/resourceclasses"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_resource_v1alpha1 "k8s.io/client-go/kubernetes/typed/resource/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) PodSchedulings(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.PodSchedulingInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodScheduling", c.clientType)
	return podschedulings.WithMetrics(c.inner.PodSchedulings(namespace), recorder)
}
func (c *withMetrics) ResourceClaimTemplates(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClaimTemplateInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ResourceClaimTemplate", c.clientType)
	return resourceclaimtemplates.WithMetrics(c.inner.ResourceClaimTemplates(namespace), recorder)
}
func (c *withMetrics) ResourceClaims(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClaimInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ResourceClaim", c.clientType)
	return resourceclaims.WithMetrics(c.inner.ResourceClaims(namespace), recorder)
}
func (c *withMetrics) ResourceClasses() k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ResourceClass", c.clientType)
	return resourceclasses.WithMetrics(c.inner.ResourceClasses(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) PodSchedulings(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.PodSchedulingInterface {
	return podschedulings.WithTracing(c.inner.PodSchedulings(namespace), c.client, "PodScheduling")
}
func (c *withTracing) ResourceClaimTemplates(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClaimTemplateInterface {
	return resourceclaimtemplates.WithTracing(c.inner.ResourceClaimTemplates(namespace), c.client, "ResourceClaimTemplate")
}
func (c *withTracing) ResourceClaims(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClaimInterface {
	return resourceclaims.WithTracing(c.inner.ResourceClaims(namespace), c.client, "ResourceClaim")
}
func (c *withTracing) ResourceClasses() k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClassInterface {
	return resourceclasses.WithTracing(c.inner.ResourceClasses(), c.client, "ResourceClass")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) PodSchedulings(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.PodSchedulingInterface {
	return podschedulings.WithLogging(c.inner.PodSchedulings(namespace), c.logger.WithValues("resource", "PodSchedulings").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceClaimTemplates(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClaimTemplateInterface {
	return resourceclaimtemplates.WithLogging(c.inner.ResourceClaimTemplates(namespace), c.logger.WithValues("resource", "ResourceClaimTemplates").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceClaims(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClaimInterface {
	return resourceclaims.WithLogging(c.inner.ResourceClaims(namespace), c.logger.WithValues("resource", "ResourceClaims").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceClasses() k8s_io_client_go_kubernetes_typed_resource_v1alpha1.ResourceClassInterface {
	return resourceclasses.WithLogging(c.inner.ResourceClasses(), c.logger.WithValues("resource", "ResourceClasses"))
}
