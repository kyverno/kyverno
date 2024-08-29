package client

import (
	"github.com/go-logr/logr"
	deviceclasses "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3/deviceclasses"
	podschedulingcontexts "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3/podschedulingcontexts"
	resourceclaims "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3/resourceclaims"
	resourceclaimtemplates "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3/resourceclaimtemplates"
	resourceslices "github.com/kyverno/kyverno/pkg/clients/kube/resourcev1alpha3/resourceslices"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_resource_v1alpha3 "k8s.io/client-go/kubernetes/typed/resource/v1alpha3"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface, client string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) DeviceClasses() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.DeviceClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "DeviceClass", c.clientType)
	return deviceclasses.WithMetrics(c.inner.DeviceClasses(), recorder)
}
func (c *withMetrics) PodSchedulingContexts(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.PodSchedulingContextInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodSchedulingContext", c.clientType)
	return podschedulingcontexts.WithMetrics(c.inner.PodSchedulingContexts(namespace), recorder)
}
func (c *withMetrics) ResourceClaimTemplates(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceClaimTemplateInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ResourceClaimTemplate", c.clientType)
	return resourceclaimtemplates.WithMetrics(c.inner.ResourceClaimTemplates(namespace), recorder)
}
func (c *withMetrics) ResourceClaims(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceClaimInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ResourceClaim", c.clientType)
	return resourceclaims.WithMetrics(c.inner.ResourceClaims(namespace), recorder)
}
func (c *withMetrics) ResourceSlices() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceSliceInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ResourceSlice", c.clientType)
	return resourceslices.WithMetrics(c.inner.ResourceSlices(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) DeviceClasses() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.DeviceClassInterface {
	return deviceclasses.WithTracing(c.inner.DeviceClasses(), c.client, "DeviceClass")
}
func (c *withTracing) PodSchedulingContexts(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.PodSchedulingContextInterface {
	return podschedulingcontexts.WithTracing(c.inner.PodSchedulingContexts(namespace), c.client, "PodSchedulingContext")
}
func (c *withTracing) ResourceClaimTemplates(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceClaimTemplateInterface {
	return resourceclaimtemplates.WithTracing(c.inner.ResourceClaimTemplates(namespace), c.client, "ResourceClaimTemplate")
}
func (c *withTracing) ResourceClaims(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceClaimInterface {
	return resourceclaims.WithTracing(c.inner.ResourceClaims(namespace), c.client, "ResourceClaim")
}
func (c *withTracing) ResourceSlices() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceSliceInterface {
	return resourceslices.WithTracing(c.inner.ResourceSlices(), c.client, "ResourceSlice")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceV1alpha3Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) DeviceClasses() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.DeviceClassInterface {
	return deviceclasses.WithLogging(c.inner.DeviceClasses(), c.logger.WithValues("resource", "DeviceClasses"))
}
func (c *withLogging) PodSchedulingContexts(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.PodSchedulingContextInterface {
	return podschedulingcontexts.WithLogging(c.inner.PodSchedulingContexts(namespace), c.logger.WithValues("resource", "PodSchedulingContexts").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceClaimTemplates(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceClaimTemplateInterface {
	return resourceclaimtemplates.WithLogging(c.inner.ResourceClaimTemplates(namespace), c.logger.WithValues("resource", "ResourceClaimTemplates").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceClaims(namespace string) k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceClaimInterface {
	return resourceclaims.WithLogging(c.inner.ResourceClaims(namespace), c.logger.WithValues("resource", "ResourceClaims").WithValues("namespace", namespace))
}
func (c *withLogging) ResourceSlices() k8s_io_client_go_kubernetes_typed_resource_v1alpha3.ResourceSliceInterface {
	return resourceslices.WithLogging(c.inner.ResourceSlices(), c.logger.WithValues("resource", "ResourceSlices"))
}
