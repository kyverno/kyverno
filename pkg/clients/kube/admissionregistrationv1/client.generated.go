package client

import (
	"github.com/go-logr/logr"
	mutatingwebhookconfigurations "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1/mutatingwebhookconfigurations"
	validatingwebhookconfigurations "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1/validatingwebhookconfigurations"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface, client string) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "MutatingWebhookConfiguration", c.clientType)
	return mutatingwebhookconfigurations.WithMetrics(c.inner.MutatingWebhookConfigurations(), recorder)
}
func (c *withMetrics) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingWebhookConfiguration", c.clientType)
	return validatingwebhookconfigurations.WithMetrics(c.inner.ValidatingWebhookConfigurations(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	return mutatingwebhookconfigurations.WithTracing(c.inner.MutatingWebhookConfigurations(), c.client, "MutatingWebhookConfiguration")
}
func (c *withTracing) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	return validatingwebhookconfigurations.WithTracing(c.inner.ValidatingWebhookConfigurations(), c.client, "ValidatingWebhookConfiguration")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_admissionregistration_v1.AdmissionregistrationV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) MutatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.MutatingWebhookConfigurationInterface {
	return mutatingwebhookconfigurations.WithLogging(c.inner.MutatingWebhookConfigurations(), c.logger.WithValues("resource", "MutatingWebhookConfigurations"))
}
func (c *withLogging) ValidatingWebhookConfigurations() k8s_io_client_go_kubernetes_typed_admissionregistration_v1.ValidatingWebhookConfigurationInterface {
	return validatingwebhookconfigurations.WithLogging(c.inner.ValidatingWebhookConfigurations(), c.logger.WithValues("resource", "ValidatingWebhookConfigurations"))
}
