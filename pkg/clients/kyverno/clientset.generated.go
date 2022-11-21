package clientset

import (
	github_com_kyverno_kyverno_pkg_client_clientset_versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	kyvernov1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1"
	kyvernov1alpha1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha2"
	kyvernov1beta1 "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1beta1"
	wgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/clients/kyverno/wgpolicyk8sv1alpha2"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/discovery"
)

type clientset struct {
	inner               github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface
	kyvernov1           github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	kyvernov1alpha1     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	kyvernov1alpha2     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	kyvernov1beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	wgpolicyk8sv1alpha2 github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
}

func (c *clientset) Discovery() discovery.DiscoveryInterface {
	return c.inner.Discovery()
}
func (c *clientset) KyvernoV1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return c.kyvernov1
}
func (c *clientset) KyvernoV1alpha1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return c.kyvernov1alpha1
}
func (c *clientset) KyvernoV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return c.kyvernov1alpha2
}
func (c *clientset) KyvernoV1beta1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return c.kyvernov1beta1
}
func (c *clientset) Wgpolicyk8sV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return c.wgpolicyk8sv1alpha2
}

func WrapWithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		inner:               inner,
		kyvernov1:           kyvernov1.WithMetrics(inner.KyvernoV1(), metrics, clientType),
		kyvernov1alpha1:     kyvernov1alpha1.WithMetrics(inner.KyvernoV1alpha1(), metrics, clientType),
		kyvernov1alpha2:     kyvernov1alpha2.WithMetrics(inner.KyvernoV1alpha2(), metrics, clientType),
		kyvernov1beta1:      kyvernov1beta1.WithMetrics(inner.KyvernoV1beta1(), metrics, clientType),
		wgpolicyk8sv1alpha2: wgpolicyk8sv1alpha2.WithMetrics(inner.Wgpolicyk8sV1alpha2(), metrics, clientType),
	}
}

func WrapWithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		inner:               inner,
		kyvernov1:           kyvernov1.WithTracing(inner.KyvernoV1(), "KyvernoV1"),
		kyvernov1alpha1:     kyvernov1alpha1.WithTracing(inner.KyvernoV1alpha1(), "KyvernoV1alpha1"),
		kyvernov1alpha2:     kyvernov1alpha2.WithTracing(inner.KyvernoV1alpha2(), "KyvernoV1alpha2"),
		kyvernov1beta1:      kyvernov1beta1.WithTracing(inner.KyvernoV1beta1(), "KyvernoV1beta1"),
		wgpolicyk8sv1alpha2: wgpolicyk8sv1alpha2.WithTracing(inner.Wgpolicyk8sV1alpha2(), "Wgpolicyk8sV1alpha2"),
	}
}
