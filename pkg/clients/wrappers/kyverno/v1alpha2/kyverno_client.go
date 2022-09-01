package v1alpha2

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"k8s.io/client-go/rest"
)

type KyvernoV1alpha2Interface interface {
	RESTClient() rest.Interface
	ClusterReportChangeRequestsGetter
	ReportChangeRequestsGetter
}

type KyvernoV1alpha2Client struct {
	restClient               rest.Interface
	kyvernov1alpha2Interface kyvernov1alpha2.KyvernoV1alpha2Interface
	clientQueryMetric        utils.ClientQueryMetric
}

func (c *KyvernoV1alpha2Client) ClusterReportChangeRequests() ClusterReportChangeRequestControlInterface {
	return newClusterReportChangeRequests(c)
}

func (c *KyvernoV1alpha2Client) ReportChangeRequests(namespace string) ReportChangeRequestControlInterface {
	return newReportChangeRequests(c, namespace)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *KyvernoV1alpha2Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func NewForConfig(restClient rest.Interface, kyvernov1alpha2Interface kyvernov1alpha2.KyvernoV1alpha2Interface, m utils.ClientQueryMetric) *KyvernoV1alpha2Client {
	return &KyvernoV1alpha2Client{restClient, kyvernov1alpha2Interface, m}
}
