package clientset

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2alpha1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	discovery "github.com/kyverno/kyverno/pkg/clients/kyverno/discovery"
	kyvernov1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha2"
	kyvernov1beta1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1beta1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2alpha1"
	kyvernov2beta1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1"
	wgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/clients/kyverno/wgpolicyk8sv1alpha2"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
)

type clientset struct {
	discovery           k8s_io_client_go_discovery.DiscoveryInterface
	kyvernov1           github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	kyvernov1alpha2     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	kyvernov1beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	kyvernov2alpha1     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface
	kyvernov2beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	wgpolicyk8sv1alpha2 github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
}

func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.discovery
}
func (c *clientset) KyvernoV1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return c.kyvernov1
}
func (c *clientset) KyvernoV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return c.kyvernov1alpha2
}
func (c *clientset) KyvernoV1beta1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return c.kyvernov1beta1
}
func (c *clientset) KyvernoV2alpha1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2alpha1.KyvernoV2alpha1Interface {
	return c.kyvernov2alpha1
}
func (c *clientset) KyvernoV2beta1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface {
	return c.kyvernov2beta1
}
func (c *clientset) Wgpolicyk8sV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return c.wgpolicyk8sv1alpha2
}

func WrapWithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, m metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		discovery:           discovery.WithMetrics(inner.Discovery(), metrics.ClusteredClientQueryRecorder(m, "Discovery", clientType)),
		kyvernov1:           kyvernov1.WithMetrics(inner.KyvernoV1(), m, clientType),
		kyvernov1alpha2:     kyvernov1alpha2.WithMetrics(inner.KyvernoV1alpha2(), m, clientType),
		kyvernov1beta1:      kyvernov1beta1.WithMetrics(inner.KyvernoV1beta1(), m, clientType),
		kyvernov2alpha1:     kyvernov2alpha1.WithMetrics(inner.KyvernoV2alpha1(), m, clientType),
		kyvernov2beta1:      kyvernov2beta1.WithMetrics(inner.KyvernoV2beta1(), m, clientType),
		wgpolicyk8sv1alpha2: wgpolicyk8sv1alpha2.WithMetrics(inner.Wgpolicyk8sV1alpha2(), m, clientType),
	}
}

func WrapWithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		discovery:           discovery.WithTracing(inner.Discovery(), "Discovery", ""),
		kyvernov1:           kyvernov1.WithTracing(inner.KyvernoV1(), "KyvernoV1"),
		kyvernov1alpha2:     kyvernov1alpha2.WithTracing(inner.KyvernoV1alpha2(), "KyvernoV1alpha2"),
		kyvernov1beta1:      kyvernov1beta1.WithTracing(inner.KyvernoV1beta1(), "KyvernoV1beta1"),
		kyvernov2alpha1:     kyvernov2alpha1.WithTracing(inner.KyvernoV2alpha1(), "KyvernoV2alpha1"),
		kyvernov2beta1:      kyvernov2beta1.WithTracing(inner.KyvernoV2beta1(), "KyvernoV2beta1"),
		wgpolicyk8sv1alpha2: wgpolicyk8sv1alpha2.WithTracing(inner.Wgpolicyk8sV1alpha2(), "Wgpolicyk8sV1alpha2"),
	}
}

func WrapWithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		discovery:           discovery.WithLogging(inner.Discovery(), logger.WithValues("group", "Discovery")),
		kyvernov1:           kyvernov1.WithLogging(inner.KyvernoV1(), logger.WithValues("group", "KyvernoV1")),
		kyvernov1alpha2:     kyvernov1alpha2.WithLogging(inner.KyvernoV1alpha2(), logger.WithValues("group", "KyvernoV1alpha2")),
		kyvernov1beta1:      kyvernov1beta1.WithLogging(inner.KyvernoV1beta1(), logger.WithValues("group", "KyvernoV1beta1")),
		kyvernov2alpha1:     kyvernov2alpha1.WithLogging(inner.KyvernoV2alpha1(), logger.WithValues("group", "KyvernoV2alpha1")),
		kyvernov2beta1:      kyvernov2beta1.WithLogging(inner.KyvernoV2beta1(), logger.WithValues("group", "KyvernoV2beta1")),
		wgpolicyk8sv1alpha2: wgpolicyk8sv1alpha2.WithLogging(inner.Wgpolicyk8sV1alpha2(), logger.WithValues("group", "Wgpolicyk8sV1alpha2")),
	}
}
