package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2alpha1"
	celpolicyexceptions "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2alpha1/celpolicyexceptions"
	globalcontextentries "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2alpha1/globalcontextentries"
	imageverificationpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2alpha1/imageverificationpolicies"
	validatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2alpha1/validatingpolicies"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CELPolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.CELPolicyExceptionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CELPolicyException", c.clientType)
	return celpolicyexceptions.WithMetrics(c.inner.CELPolicyExceptions(namespace), recorder)
}
func (c *withMetrics) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.GlobalContextEntryInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "GlobalContextEntry", c.clientType)
	return globalcontextentries.WithMetrics(c.inner.GlobalContextEntries(), recorder)
}
func (c *withMetrics) ImageVerificationPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ImageVerificationPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ImageVerificationPolicy", c.clientType)
	return imageverificationpolicies.WithMetrics(c.inner.ImageVerificationPolicies(), recorder)
}
func (c *withMetrics) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ValidatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingPolicy", c.clientType)
	return validatingpolicies.WithMetrics(c.inner.ValidatingPolicies(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CELPolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.CELPolicyExceptionInterface {
	return celpolicyexceptions.WithTracing(c.inner.CELPolicyExceptions(namespace), c.client, "CELPolicyException")
}
func (c *withTracing) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.GlobalContextEntryInterface {
	return globalcontextentries.WithTracing(c.inner.GlobalContextEntries(), c.client, "GlobalContextEntry")
}
func (c *withTracing) ImageVerificationPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ImageVerificationPolicyInterface {
	return imageverificationpolicies.WithTracing(c.inner.ImageVerificationPolicies(), c.client, "ImageVerificationPolicy")
}
func (c *withTracing) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ValidatingPolicyInterface {
	return validatingpolicies.WithTracing(c.inner.ValidatingPolicies(), c.client, "ValidatingPolicy")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CELPolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.CELPolicyExceptionInterface {
	return celpolicyexceptions.WithLogging(c.inner.CELPolicyExceptions(namespace), c.logger.WithValues("resource", "CELPolicyExceptions").WithValues("namespace", namespace))
}
func (c *withLogging) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.GlobalContextEntryInterface {
	return globalcontextentries.WithLogging(c.inner.GlobalContextEntries(), c.logger.WithValues("resource", "GlobalContextEntries"))
}
func (c *withLogging) ImageVerificationPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ImageVerificationPolicyInterface {
	return imageverificationpolicies.WithLogging(c.inner.ImageVerificationPolicies(), c.logger.WithValues("resource", "ImageVerificationPolicies"))
}
func (c *withLogging) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ValidatingPolicyInterface {
	return validatingpolicies.WithLogging(c.inner.ValidatingPolicies(), c.logger.WithValues("resource", "ValidatingPolicies"))
}
