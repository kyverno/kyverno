package resource

import (
	"context"

	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func NewFakeHandlers(ctx context.Context, policyCache policycache.Cache) webhooks.Handlers {

	client := fake.NewSimpleClientset()
	metricsConfig := metrics.NewFakeMetricsConfig(client)

	informers := informers.NewSharedInformerFactory(client, 0)
	informers.Start(ctx.Done())

	kyvernoclient := fakekyvernov1.NewSimpleClientset()
	kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(kyvernoclient, 0)
	kyvernoInformers.Start(ctx.Done())

	return &handlers{
		client:            dclient.NewEmptyFakeClient(),
		kyvernoClient:     fakekyvernov1.NewSimpleClientset(),
		configuration:     config.NewFakeConfig(),
		metricsConfig:     metricsConfig,
		pCache:            policyCache,
		nsLister:          informers.Core().V1().Namespaces().Lister(),
		rbLister:          informers.Rbac().V1().RoleBindings().Lister(),
		crbLister:         informers.Rbac().V1().ClusterRoleBindings().Lister(),
		urLister:          kyvernoInformers.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
		prGenerator:       policyreport.NewFake(),
		urGenerator:       updaterequest.NewFake(),
		eventGen:          event.NewFake(),
		auditHandler:      newFakeAuditHandler(),
		openAPIController: openapi.NewFake(),
	}
}

func newFakeAuditHandler() AuditHandler {
	return &fakeAuditHandler{}
}

type fakeAuditHandler struct {
}

func (f *fakeAuditHandler) Add(request *admissionv1.AdmissionRequest) {

}

func (f *fakeAuditHandler) Run(workers int, stopCh <-chan struct{}) {

}
