package kyvernoclient

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	versionedkyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	versionedkyvernov1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	versionedkyvernov1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	versionedpolicyreportv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	wrappedkyvernov1 "github.com/kyverno/kyverno/pkg/clients/wrappers/kyverno/v1"
	wrappedkyvernov1alpha2 "github.com/kyverno/kyverno/pkg/clients/wrappers/kyverno/v1alpha2"
	wrappedkyvernov1beta1 "github.com/kyverno/kyverno/pkg/clients/wrappers/kyverno/v1beta1"
	wrappedwgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/clients/wrappers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

type clientset struct {
	versioned.Interface
	kyvernoV1           versionedkyvernov1.KyvernoV1Interface
	kyvernoV1beta1      versionedkyvernov1beta1.KyvernoV1beta1Interface
	kyvernoV1alpha2     versionedkyvernov1alpha2.KyvernoV1alpha2Interface
	wgpolicyk8sV1alpha2 versionedpolicyreportv1alpha2.Wgpolicyk8sV1alpha2Interface
}

func (c *clientset) KyvernoV1() versionedkyvernov1.KyvernoV1Interface {
	return c.kyvernoV1
}

func (c *clientset) KyvernoV1beta1() versionedkyvernov1beta1.KyvernoV1beta1Interface {
	return c.kyvernoV1beta1
}

func (c *clientset) KyvernoV1alpha2() versionedkyvernov1alpha2.KyvernoV1alpha2Interface {
	return c.kyvernoV1alpha2
}

func (c *clientset) Wgpolicyk8sV1alpha2() versionedpolicyreportv1alpha2.Wgpolicyk8sV1alpha2Interface {
	return c.wgpolicyk8sV1alpha2
}

func NewForConfig(c *rest.Config, m metrics.MetricsConfigManager) (versioned.Interface, error) {
	kClientset, err := versioned.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return &clientset{
		Interface:           kClientset,
		kyvernoV1:           wrappedkyvernov1.Wrap(kClientset.KyvernoV1(), m),
		kyvernoV1beta1:      wrappedkyvernov1beta1.Wrap(kClientset.KyvernoV1beta1(), m),
		kyvernoV1alpha2:     wrappedkyvernov1alpha2.Wrap(kClientset.KyvernoV1alpha2(), m),
		wgpolicyk8sV1alpha2: wrappedwgpolicyk8sv1alpha2.Wrap(kClientset.Wgpolicyk8sV1alpha2(), m),
	}, nil
}
