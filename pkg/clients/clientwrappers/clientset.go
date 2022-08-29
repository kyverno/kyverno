package kyvernoclient

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/kyverno/kyverno/pkg/clients/clientwrappers/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/pkg/clients/clientwrappers/kyverno/v1alpha2"
	kyvernov1beta1 "github.com/kyverno/kyverno/pkg/clients/clientwrappers/kyverno/v1beta1"
	wgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/clients/clientwrappers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/clientwrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

type Interface interface {
	KyvernoV1() kyvernov1.KyvernoV1Interface
	KyvernoV1beta1() kyvernov1beta1.KyvernoV1beta1Interface
	KyvernoV1alpha2() kyvernov1alpha2.KyvernoV1alpha2Interface
	Wgpolicyk8sV1alpha2() wgpolicyk8sv1alpha2.Wgpolicyk8sV1alpha2Interface
}

type Clientset struct {
	kyvernoV1           *kyvernov1.KyvernoV1Client
	kyvernoV1beta1      *kyvernov1beta1.KyvernoV1beta1Client
	kyvernoV1alpha2     *kyvernov1alpha2.KyvernoV1alpha2Client
	wgpolicyk8sV1alpha2 *wgpolicyk8sv1alpha2.Wgpolicyk8sV1alpha2Client
}

func (c *Clientset) KyvernoV1() kyvernov1.KyvernoV1Interface {
	return c.kyvernoV1
}

func (c *Clientset) KyvernoV1beta1() kyvernov1beta1.KyvernoV1beta1Interface {
	return c.kyvernoV1beta1
}

func (c *Clientset) KyvernoV1alpha2() kyvernov1alpha2.KyvernoV1alpha2Interface {
	return c.kyvernoV1alpha2
}

func (c *Clientset) Wgpolicyk8sV1alpha2() wgpolicyk8sv1alpha2.Wgpolicyk8sV1alpha2Interface {
	return c.wgpolicyk8sV1alpha2
}

func NewForConfig(c *rest.Config, m *metrics.MetricsConfig) (*Clientset, error) {
	var cs Clientset
	clientQueryMetric := utils.NewClientQueryMetric(m)

	kClientset, err := versioned.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	cs.kyvernoV1 = kyvernov1.NewForConfig(
		kClientset.KyvernoV1().RESTClient(),
		kClientset.KyvernoV1(),
		clientQueryMetric)

	cs.kyvernoV1beta1 = kyvernov1beta1.NewForConfig(
		kClientset.KyvernoV1beta1().RESTClient(),
		kClientset.KyvernoV1beta1(),
		clientQueryMetric)

	cs.kyvernoV1alpha2 = kyvernov1alpha2.NewForConfig(
		kClientset.KyvernoV1alpha2().RESTClient(),
		kClientset.KyvernoV1alpha2(),
		clientQueryMetric)

	cs.wgpolicyk8sV1alpha2 = wgpolicyk8sv1alpha2.NewForConfig(
		kClientset.Wgpolicyk8sV1alpha2().RESTClient(),
		kClientset.Wgpolicyk8sV1alpha2(),
		clientQueryMetric)

	return &cs, nil
}
