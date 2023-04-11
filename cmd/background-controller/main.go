package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/background"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	dynamicclient "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	"github.com/kyverno/kyverno/pkg/config"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	"github.com/kyverno/kyverno/pkg/cosign"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

const (
	resyncPeriod = 15 * time.Minute
)

func setupRegistryClient(ctx context.Context, logger logr.Logger, lister corev1listers.SecretNamespaceLister, imagePullSecrets string, allowInsecureRegistry bool) (registryclient.Client, error) {
	logger = logger.WithName("registry-client")
	logger.Info("setup registry client...", "secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	registryOptions := []registryclient.Option{
		registryclient.WithTracing(),
	}
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(ctx, lister, secrets...))
	}
	if allowInsecureRegistry {
		registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
	}
	return registryclient.New(registryOptions...)
}

func setupCosign(logger logr.Logger, imageSignatureRepository string) {
	logger = logger.WithName("cosign")
	logger.Info("setup cosign...", "repository", imageSignatureRepository)
	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}
}

func createrLeaderControllers(
	eng engineapi.Engine,
	genWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	rclient registryclient.Client,
	configuration config.Configuration,
	metricsConfig metrics.MetricsConfigManager,
	eventGenerator event.Interface,
) ([]internal.Controller, error) {
	policyCtrl, err := policy.NewPolicyController(
		kyvernoClient,
		dynamicClient,
		eng,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		configuration,
		eventGenerator,
		kubeInformer.Core().V1().Namespaces(),
		logging.WithName("PolicyController"),
		time.Hour,
		metricsConfig,
	)
	if err != nil {
		return nil, err
	}
	backgroundController := background.NewController(
		kyvernoClient,
		dynamicClient,
		eng,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		eventGenerator,
		configuration,
	)
	return []internal.Controller{
		internal.NewController("policy-controller", policyCtrl, 2),
		internal.NewController("background-controller", backgroundController, genWorkers),
	}, err
}

func main() {
	var (
		genWorkers                int
		maxQueuedEvents           int
		imagePullSecrets          string
		imageSignatureRepository  string
		allowInsecureRegistry     bool
		leaderElectionRetryPeriod time.Duration
	)
	flagset := flag.NewFlagSet("updaterequest-controller", flag.ExitOnError)
	flagset.IntVar(&genWorkers, "genWorkers", 10, "Workers for the background controller.")
	flagset.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flagset.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flagset.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.DurationVar(&leaderElectionRetryPeriod, "leaderElectionRetryPeriod", leaderelection.DefaultRetryPeriod, "Configure leader election retry period.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithPolicyExceptions(),
		internal.WithConfigMapCaching(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup logger
	// show version
	// start profiling
	// setup signals
	// setup maxprocs
	// setup metrics
	signalCtx, setup, sdown := internal.Setup("kyverno-background-controller", false)
	defer sdown()
	// logger := setup.Logger
	// create instrumented clients
	leaderElectionClient := internal.CreateKubernetesClient(setup.Logger, kubeclient.WithMetrics(setup.MetricsManager, metrics.KubeClient), kubeclient.WithTracing())
	kyvernoClient := internal.CreateKyvernoClient(setup.Logger, kyvernoclient.WithMetrics(setup.MetricsManager, metrics.KyvernoClient), kyvernoclient.WithTracing())
	dynamicClient := internal.CreateDynamicClient(setup.Logger, dynamicclient.WithMetrics(setup.MetricsManager, metrics.KyvernoClient), dynamicclient.WithTracing())
	dClient, err := dclient.NewClient(signalCtx, dynamicClient, setup.KubeClient, 15*time.Minute)
	if err != nil {
		setup.Logger.Error(err, "failed to create dynamic client")
		os.Exit(1)
	}
	// THIS IS AN UGLY FIX
	// ELSE KYAML IS NOT THREAD SAFE
	kyamlopenapi.Schema()
	// informer factories
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
	// setup registry client
	rclient, err := setupRegistryClient(signalCtx, setup.Logger, secretLister, imagePullSecrets, allowInsecureRegistry)
	if err != nil {
		setup.Logger.Error(err, "failed to setup registry client")
		os.Exit(1)
	}
	// setup cosign
	setupCosign(setup.Logger, imageSignatureRepository)
	eventGenerator := event.NewEventGenerator(
		dClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		logging.WithName("EventGenerator"),
	)
	// this controller only subscribe to events, nothing is returned...
	var wg sync.WaitGroup
	policymetricscontroller.NewController(
		setup.MetricsManager,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		&wg,
	)
	engine := internal.NewEngine(
		signalCtx,
		setup.Logger,
		setup.Configuration,
		setup.MetricsConfiguration,
		dClient,
		rclient,
		setup.KubeClient,
		kyvernoClient,
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer, kubeKyvernoInformer) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// start event generator
	go eventGenerator.Run(signalCtx, 3, &wg)
	// setup leader election
	le, err := leaderelection.New(
		setup.Logger.WithName("leader-election"),
		"kyverno-background-controller",
		config.KyvernoNamespace(),
		leaderElectionClient,
		config.KyvernoPodName(),
		leaderElectionRetryPeriod,
		func(ctx context.Context) {
			logger := setup.Logger.WithName("leader")
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, resyncPeriod)
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
			// create leader controllers
			leaderControllers, err := createrLeaderControllers(
				engine,
				genWorkers,
				kubeInformer,
				kyvernoInformer,
				kyvernoClient,
				dClient,
				rclient,
				setup.Configuration,
				setup.MetricsManager,
				eventGenerator,
			)
			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(signalCtx, logger, kyvernoInformer, kubeInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			// start leader controllers
			var wg sync.WaitGroup
			for _, controller := range leaderControllers {
				controller.Run(signalCtx, logger.WithName("controllers"), &wg)
			}
			// wait all controllers shut down
			wg.Wait()
		},
		nil,
	)
	if err != nil {
		setup.Logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// start leader election
	le.Run(signalCtx)
	wg.Wait()
}
