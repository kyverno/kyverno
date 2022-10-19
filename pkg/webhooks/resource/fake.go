package resource

import (
	"context"

	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	policyexceptions "github.com/kyverno/kyverno/pkg/policyexceptions"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func NewFakeHandlers(ctx context.Context, policyCache policycache.Cache) webhooks.ResourceHandlers {
	client := fake.NewSimpleClientset()
	metricsConfig := metrics.NewFakeMetricsConfig()

	informers := kubeinformers.NewSharedInformerFactory(client, 0)
	informers.Start(ctx.Done())

	kyvernoclient := fakekyvernov1.NewSimpleClientset()
	kyvernoInformers := kyvernoinformers.NewSharedInformerFactory(kyvernoclient, 0)
	configMapResolver, _ := resolvers.NewClientBasedResolver(client)
	kyvernoInformers.Start(ctx.Done())

	dclient := dclient.NewEmptyFakeClient()
	configuration := config.NewDefaultConfiguration()
	rbLister := informers.Rbac().V1().RoleBindings().Lister()
	crbLister := informers.Rbac().V1().ClusterRoleBindings().Lister()
	urLister := kyvernoInformers.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace())
	mockException := policyexceptions.NewMockPolicyException()

	return &handlers{
		client:         dclient,
		configuration:  configuration,
		metricsConfig:  metricsConfig,
		pCache:         policyCache,
		nsLister:       informers.Core().V1().Namespaces().Lister(),
		rbLister:       rbLister,
		crbLister:      crbLister,
		urLister:       urLister,
		urGenerator:    updaterequest.NewFake(),
		eventGen:       event.NewFake(),
		openApiManager: openapi.NewFake(),
		pcBuilder:      webhookutils.NewPolicyContextBuilder(configuration, dclient, rbLister, crbLister, configMapResolver, mockException),
		urUpdater:      webhookutils.NewUpdateRequestUpdater(kyvernoclient, urLister),
	}
}
