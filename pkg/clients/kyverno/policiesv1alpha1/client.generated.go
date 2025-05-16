package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policies.kyverno.io/v1alpha1"
	imagevalidatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1alpha1/imagevalidatingpolicies"
	policyexceptions "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1alpha1/policyexceptions"
	validatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1alpha1/validatingpolicies"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ImageValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.ImageValidatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ImageValidatingPolicy", c.clientType)
	return imagevalidatingpolicies.WithMetrics(c.inner.ImageValidatingPolicies(), recorder)
}
func (c *withMetrics) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PolicyExceptionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyException", c.clientType)
	return policyexceptions.WithMetrics(c.inner.PolicyExceptions(namespace), recorder)
}
func (c *withMetrics) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.ValidatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingPolicy", c.clientType)
	return validatingpolicies.WithMetrics(c.inner.ValidatingPolicies(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ImageValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.ImageValidatingPolicyInterface {
	return imagevalidatingpolicies.WithTracing(c.inner.ImageValidatingPolicies(), c.client, "ImageValidatingPolicy")
}
func (c *withTracing) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PolicyExceptionInterface {
	return policyexceptions.WithTracing(c.inner.PolicyExceptions(namespace), c.client, "PolicyException")
}
func (c *withTracing) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.ValidatingPolicyInterface {
	return validatingpolicies.WithTracing(c.inner.ValidatingPolicies(), c.client, "ValidatingPolicy")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PoliciesV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ImageValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.ImageValidatingPolicyInterface {
	return imagevalidatingpolicies.WithLogging(c.inner.ImageValidatingPolicies(), c.logger.WithValues("resource", "ImageValidatingPolicies"))
}
func (c *withLogging) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.PolicyExceptionInterface {
	return policyexceptions.WithLogging(c.inner.PolicyExceptions(namespace), c.logger.WithValues("resource", "PolicyExceptions").WithValues("namespace", namespace))
}
func (c *withLogging) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1alpha1.ValidatingPolicyInterface {
	return validatingpolicies.WithLogging(c.inner.ValidatingPolicies(), c.logger.WithValues("resource", "ValidatingPolicies"))
}
