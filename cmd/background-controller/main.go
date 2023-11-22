package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/background"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeinformers "k8s.io/client-go/informers"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

const (
	resyncPeriod = 15 * time.Minute
)

func createrLeaderControllers(
	eng engineapi.Engine,
	genWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	configuration config.Configuration,
	metricsConfig metrics.MetricsConfigManager,
	eventGenerator event.Interface,
	jp jmespath.Interface,
	backgroundScanInterval time.Duration,
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
		backgroundScanInterval,
		metricsConfig,
		jp,
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
		jp,
	)
	return []internal.Controller{
		internal.NewController("policy-controller", policyCtrl, 2),
		internal.NewController("background-controller", backgroundController, genWorkers),
	}, err
}

func main() {
	var (
		genWorkers               int
		maxQueuedEvents          int
		omitEvents               string
		maxAPICallResponseLength int64
	)
	flagset := flag.NewFlagSet("updaterequest-controller", flag.ExitOnError)
	flagset.IntVar(&genWorkers, "genWorkers", 10, "Workers for the background controller.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omit-events", "", "Set this flag to a comma sperated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omit-events=PolicyApplied,PolicyViolation")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 2*1000*1000, "Maximum allowed response size from API Calls. A value of 0 bypasses checks (not recommended).")

	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithPolicyExceptions(),
		internal.WithConfigMapCaching(),
		internal.WithDeferredLoading(),
		internal.WithRegistryClient(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithDynamicClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup
	signalCtx, setup, sdown := internal.Setup(appConfig, "kyverno-background-controller", false)
	defer sdown()

	var err error
	bgscanInterval := time.Hour
	val := os.Getenv("BACKGROUND_SCAN_INTERVAL")
	if val != "" {
		if bgscanInterval, err = time.ParseDuration(val); err != nil {
			setup.Logger.Error(err, "failed to set the background scan interval")
			os.Exit(1)
		}
	}
	setup.Logger.V(2).Info("setting the background scan interval", "value", bgscanInterval.String())

	// THIS IS AN UGLY FIX
	// ELSE KYAML IS NOT THREAD SAFE
	kyamlopenapi.Schema()
	// informer factories
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
	emitEventsValues := strings.Split(omitEvents, ",")
	if omitEvents == "" {
		emitEventsValues = []string{}
	}
	eventGenerator := event.NewEventGenerator(
		setup.KyvernoDynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		emitEventsValues,
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
		setup.Jp,
		setup.KyvernoDynamicClient,
		setup.RegistryClient,
		setup.ImageVerifyCacheClient,
		setup.KubeClient,
		setup.KyvernoClient,
		setup.RegistrySecretLister,
		apicall.NewAPICallConfiguration(maxAPICallResponseLength),
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer) {
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
		setup.LeaderElectionClient,
		config.KyvernoPodName(),
		internal.LeaderElectionRetryPeriod(),
		func(ctx context.Context) {
			logger := setup.Logger.WithName("leader")
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, resyncPeriod)
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
			// create leader controllers
			leaderControllers, err := createrLeaderControllers(
				engine,
				genWorkers,
				kubeInformer,
				kyvernoInformer,
				setup.KyvernoClient,
				setup.KyvernoDynamicClient,
				setup.Configuration,
				setup.MetricsManager,
				eventGenerator,
				setup.Jp,
				bgscanInterval,
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
