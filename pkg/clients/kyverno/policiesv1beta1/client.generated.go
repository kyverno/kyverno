package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policies.kyverno.io/v1beta1"
	deletingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/deletingpolicies"
	generatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/generatingpolicies"
	imagevalidatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/imagevalidatingpolicies"
	mutatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/mutatingpolicies"
	namespaceddeletingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/namespaceddeletingpolicies"
	namespacedgeneratingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/namespacedgeneratingpolicies"
	namespacedimagevalidatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/namespacedimagevalidatingpolicies"
	namespacedmutatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/namespacedmutatingpolicies"
	namespacedvalidatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/namespacedvalidatingpolicies"
	policyexceptions "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/policyexceptions"
	validatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/validatingpolicies"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) DeletingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.DeletingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "DeletingPolicy", c.clientType)
	return deletingpolicies.WithMetrics(c.inner.DeletingPolicies(), recorder)
}
func (c *withMetrics) GeneratingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.GeneratingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "GeneratingPolicy", c.clientType)
	return generatingpolicies.WithMetrics(c.inner.GeneratingPolicies(), recorder)
}
func (c *withMetrics) ImageValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ImageValidatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ImageValidatingPolicy", c.clientType)
	return imagevalidatingpolicies.WithMetrics(c.inner.ImageValidatingPolicies(), recorder)
}
func (c *withMetrics) MutatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.MutatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "MutatingPolicy", c.clientType)
	return mutatingpolicies.WithMetrics(c.inner.MutatingPolicies(), recorder)
}
func (c *withMetrics) NamespacedDeletingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedDeletingPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NamespacedDeletingPolicy", c.clientType)
	return namespaceddeletingpolicies.WithMetrics(c.inner.NamespacedDeletingPolicies(namespace), recorder)
}
func (c *withMetrics) NamespacedGeneratingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedGeneratingPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NamespacedGeneratingPolicy", c.clientType)
	return namespacedgeneratingpolicies.WithMetrics(c.inner.NamespacedGeneratingPolicies(namespace), recorder)
}
func (c *withMetrics) NamespacedImageValidatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedImageValidatingPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NamespacedImageValidatingPolicy", c.clientType)
	return namespacedimagevalidatingpolicies.WithMetrics(c.inner.NamespacedImageValidatingPolicies(namespace), recorder)
}
func (c *withMetrics) NamespacedMutatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedMutatingPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NamespacedMutatingPolicy", c.clientType)
	return namespacedmutatingpolicies.WithMetrics(c.inner.NamespacedMutatingPolicies(namespace), recorder)
}
func (c *withMetrics) NamespacedValidatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedValidatingPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NamespacedValidatingPolicy", c.clientType)
	return namespacedvalidatingpolicies.WithMetrics(c.inner.NamespacedValidatingPolicies(namespace), recorder)
}
func (c *withMetrics) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PolicyExceptionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyException", c.clientType)
	return policyexceptions.WithMetrics(c.inner.PolicyExceptions(namespace), recorder)
}
func (c *withMetrics) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ValidatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingPolicy", c.clientType)
	return validatingpolicies.WithMetrics(c.inner.ValidatingPolicies(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) DeletingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.DeletingPolicyInterface {
	return deletingpolicies.WithTracing(c.inner.DeletingPolicies(), c.client, "DeletingPolicy")
}
func (c *withTracing) GeneratingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.GeneratingPolicyInterface {
	return generatingpolicies.WithTracing(c.inner.GeneratingPolicies(), c.client, "GeneratingPolicy")
}
func (c *withTracing) ImageValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ImageValidatingPolicyInterface {
	return imagevalidatingpolicies.WithTracing(c.inner.ImageValidatingPolicies(), c.client, "ImageValidatingPolicy")
}
func (c *withTracing) MutatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.MutatingPolicyInterface {
	return mutatingpolicies.WithTracing(c.inner.MutatingPolicies(), c.client, "MutatingPolicy")
}
func (c *withTracing) NamespacedDeletingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedDeletingPolicyInterface {
	return namespaceddeletingpolicies.WithTracing(c.inner.NamespacedDeletingPolicies(namespace), c.client, "NamespacedDeletingPolicy")
}
func (c *withTracing) NamespacedGeneratingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedGeneratingPolicyInterface {
	return namespacedgeneratingpolicies.WithTracing(c.inner.NamespacedGeneratingPolicies(namespace), c.client, "NamespacedGeneratingPolicy")
}
func (c *withTracing) NamespacedImageValidatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedImageValidatingPolicyInterface {
	return namespacedimagevalidatingpolicies.WithTracing(c.inner.NamespacedImageValidatingPolicies(namespace), c.client, "NamespacedImageValidatingPolicy")
}
func (c *withTracing) NamespacedMutatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedMutatingPolicyInterface {
	return namespacedmutatingpolicies.WithTracing(c.inner.NamespacedMutatingPolicies(namespace), c.client, "NamespacedMutatingPolicy")
}
func (c *withTracing) NamespacedValidatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedValidatingPolicyInterface {
	return namespacedvalidatingpolicies.WithTracing(c.inner.NamespacedValidatingPolicies(namespace), c.client, "NamespacedValidatingPolicy")
}
func (c *withTracing) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PolicyExceptionInterface {
	return policyexceptions.WithTracing(c.inner.PolicyExceptions(namespace), c.client, "PolicyException")
}
func (c *withTracing) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ValidatingPolicyInterface {
	return validatingpolicies.WithTracing(c.inner.ValidatingPolicies(), c.client, "ValidatingPolicy")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) DeletingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.DeletingPolicyInterface {
	return deletingpolicies.WithLogging(c.inner.DeletingPolicies(), c.logger.WithValues("resource", "DeletingPolicies"))
}
func (c *withLogging) GeneratingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.GeneratingPolicyInterface {
	return generatingpolicies.WithLogging(c.inner.GeneratingPolicies(), c.logger.WithValues("resource", "GeneratingPolicies"))
}
func (c *withLogging) ImageValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ImageValidatingPolicyInterface {
	return imagevalidatingpolicies.WithLogging(c.inner.ImageValidatingPolicies(), c.logger.WithValues("resource", "ImageValidatingPolicies"))
}
func (c *withLogging) MutatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.MutatingPolicyInterface {
	return mutatingpolicies.WithLogging(c.inner.MutatingPolicies(), c.logger.WithValues("resource", "MutatingPolicies"))
}
func (c *withLogging) NamespacedDeletingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedDeletingPolicyInterface {
	return namespaceddeletingpolicies.WithLogging(c.inner.NamespacedDeletingPolicies(namespace), c.logger.WithValues("resource", "NamespacedDeletingPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) NamespacedGeneratingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedGeneratingPolicyInterface {
	return namespacedgeneratingpolicies.WithLogging(c.inner.NamespacedGeneratingPolicies(namespace), c.logger.WithValues("resource", "NamespacedGeneratingPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) NamespacedImageValidatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedImageValidatingPolicyInterface {
	return namespacedimagevalidatingpolicies.WithLogging(c.inner.NamespacedImageValidatingPolicies(namespace), c.logger.WithValues("resource", "NamespacedImageValidatingPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) NamespacedMutatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedMutatingPolicyInterface {
	return namespacedmutatingpolicies.WithLogging(c.inner.NamespacedMutatingPolicies(namespace), c.logger.WithValues("resource", "NamespacedMutatingPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) NamespacedValidatingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedValidatingPolicyInterface {
	return namespacedvalidatingpolicies.WithLogging(c.inner.NamespacedValidatingPolicies(namespace), c.logger.WithValues("resource", "NamespacedValidatingPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PolicyExceptionInterface {
	return policyexceptions.WithLogging(c.inner.PolicyExceptions(namespace), c.logger.WithValues("resource", "PolicyExceptions").WithValues("namespace", namespace))
}
func (c *withLogging) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ValidatingPolicyInterface {
	return validatingpolicies.WithLogging(c.inner.ValidatingPolicies(), c.logger.WithValues("resource", "ValidatingPolicies"))
}
