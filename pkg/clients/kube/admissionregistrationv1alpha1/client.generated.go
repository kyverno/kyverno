package client

import (
	"github.com/go-logr/logr"
	mutatingadmissionpolicies "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1alpha1/mutatingadmissionpolicies"
	mutatingadmissionpolicybindings "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1alpha1/mutatingadmissionpolicybindings"
	validatingadmissionpolicies "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1alpha1/validatingadmissionpolicies"
	validatingadmissionpolicybindings "github.com/kyverno/kyverno/pkg/clients/kube/admissionregistrationv1alpha1/validatingadmissionpolicybindings"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) MutatingAdmissionPolicies() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.MutatingAdmissionPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "MutatingAdmissionPolicy", c.clientType)
	return mutatingadmissionpolicies.WithMetrics(c.inner.MutatingAdmissionPolicies(), recorder)
}
func (c *withMetrics) MutatingAdmissionPolicyBindings() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.MutatingAdmissionPolicyBindingInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "MutatingAdmissionPolicyBinding", c.clientType)
	return mutatingadmissionpolicybindings.WithMetrics(c.inner.MutatingAdmissionPolicyBindings(), recorder)
}
func (c *withMetrics) ValidatingAdmissionPolicies() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingAdmissionPolicy", c.clientType)
	return validatingadmissionpolicies.WithMetrics(c.inner.ValidatingAdmissionPolicies(), recorder)
}
func (c *withMetrics) ValidatingAdmissionPolicyBindings() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingAdmissionPolicyBinding", c.clientType)
	return validatingadmissionpolicybindings.WithMetrics(c.inner.ValidatingAdmissionPolicyBindings(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) MutatingAdmissionPolicies() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.MutatingAdmissionPolicyInterface {
	return mutatingadmissionpolicies.WithTracing(c.inner.MutatingAdmissionPolicies(), c.client, "MutatingAdmissionPolicy")
}
func (c *withTracing) MutatingAdmissionPolicyBindings() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.MutatingAdmissionPolicyBindingInterface {
	return mutatingadmissionpolicybindings.WithTracing(c.inner.MutatingAdmissionPolicyBindings(), c.client, "MutatingAdmissionPolicyBinding")
}
func (c *withTracing) ValidatingAdmissionPolicies() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyInterface {
	return validatingadmissionpolicies.WithTracing(c.inner.ValidatingAdmissionPolicies(), c.client, "ValidatingAdmissionPolicy")
}
func (c *withTracing) ValidatingAdmissionPolicyBindings() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface {
	return validatingadmissionpolicybindings.WithTracing(c.inner.ValidatingAdmissionPolicyBindings(), c.client, "ValidatingAdmissionPolicyBinding")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.AdmissionregistrationV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) MutatingAdmissionPolicies() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.MutatingAdmissionPolicyInterface {
	return mutatingadmissionpolicies.WithLogging(c.inner.MutatingAdmissionPolicies(), c.logger.WithValues("resource", "MutatingAdmissionPolicies"))
}
func (c *withLogging) MutatingAdmissionPolicyBindings() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.MutatingAdmissionPolicyBindingInterface {
	return mutatingadmissionpolicybindings.WithLogging(c.inner.MutatingAdmissionPolicyBindings(), c.logger.WithValues("resource", "MutatingAdmissionPolicyBindings"))
}
func (c *withLogging) ValidatingAdmissionPolicies() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyInterface {
	return validatingadmissionpolicies.WithLogging(c.inner.ValidatingAdmissionPolicies(), c.logger.WithValues("resource", "ValidatingAdmissionPolicies"))
}
func (c *withLogging) ValidatingAdmissionPolicyBindings() k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface {
	return validatingadmissionpolicybindings.WithLogging(c.inner.ValidatingAdmissionPolicyBindings(), c.logger.WithValues("resource", "ValidatingAdmissionPolicyBindings"))
}
