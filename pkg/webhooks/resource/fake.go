package resource

import (
	"context"

	fakekyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
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
	configuration := config.NewDefaultConfiguration(false)
	urLister := kyvernoInformers.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace())
	peLister := kyvernoInformers.Kyverno().V2beta1().PolicyExceptions().Lister()
	jp := jmespath.New(configuration)
	rclient := registryclient.NewOrDie()

	return &resourceHandlers{
		client:        dclient,
		configuration: configuration,
		metricsConfig: metricsConfig,
		pCache:        policyCache,
		nsLister:      informers.Core().V1().Namespaces().Lister(),
		urLister:      urLister,
		urGenerator:   updaterequest.NewFake(),
		eventGen:      event.NewFake(),
		pcBuilder:     webhookutils.NewPolicyContextBuilder(configuration, jp),
		engine: engine.NewEngine(
			configuration,
			config.NewDefaultMetricsConfiguration(),
			jp,
			adapters.Client(dclient),
			factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
			imageverifycache.DisabledImageVerifyCache(),
			factories.DefaultContextLoaderFactory(configMapResolver),
			peLister,
			"",
		),
	}
}
