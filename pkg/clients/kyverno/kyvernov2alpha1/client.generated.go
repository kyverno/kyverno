package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2alpha1"
	globalcontextentries "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2alpha1/globalcontextentries"
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
func (c *withMetrics) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.GlobalContextEntryInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "GlobalContextEntry", c.clientType)
	return globalcontextentries.WithMetrics(c.inner.GlobalContextEntries(), recorder)
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
func (c *withTracing) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.GlobalContextEntryInterface {
	return globalcontextentries.WithTracing(c.inner.GlobalContextEntries(), c.client, "GlobalContextEntry")
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
func (c *withLogging) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.GlobalContextEntryInterface {
	return globalcontextentries.WithLogging(c.inner.GlobalContextEntries(), c.logger.WithValues("resource", "GlobalContextEntries"))
}
func (c *withLogging) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.ValidatingPolicyInterface {
	return validatingpolicies.WithLogging(c.inner.ValidatingPolicies(), c.logger.WithValues("resource", "ValidatingPolicies"))
}
