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
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	globalcontextcontroller "github.com/kyverno/kyverno/pkg/controllers/globalcontext"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/utils/generator"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/client-go/informers"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

func sanityChecks(apiserverClient apiserver.Interface) error {
	return kubeutils.CRDsInstalled(apiserverClient, "updaterequests.kyverno.io")
}

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
	urGenerator generator.UpdateRequestGenerator,
	reportsConfig reportutils.ReportingConfiguration,
	reportsBreaker breaker.Breaker,
) ([]internal.Controller, error) {
	policyCtrl, err := policy.NewPolicyController(
		kyvernoClient,
		dynamicClient,
		eng,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V2().UpdateRequests(),
		configuration,
		eventGenerator,
		kubeInformer.Core().V1().Namespaces(),
		logging.WithName("PolicyController"),
		backgroundScanInterval,
		metricsConfig,
		jp,
		urGenerator,
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
		kyvernoInformer.Kyverno().V2().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		eventGenerator,
		configuration,
		jp,
		reportsConfig,
		reportsBreaker,
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
		maxBackgroundReports     int
	)
	flagset := flag.NewFlagSet("updaterequest-controller", flag.ExitOnError)
	flagset.IntVar(&genWorkers, "genWorkers", 10, "Workers for the background controller.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omitEvents", "", "Set this flag to a comma sperated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omitEvents=PolicyApplied,PolicyViolation")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 2*1000*1000, "Maximum allowed response size from API Calls. A value of 0 bypasses checks (not recommended).")
	flagset.IntVar(&maxBackgroundReports, "maxBackgroundReports", 10000, "Maximum number of ephemeralreports created for the background policies.")

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
		internal.WithEventsClient(),
		internal.WithApiServerClient(),
		internal.WithMetadataClient(),
		internal.WithFlagSets(flagset),
		internal.WithReporting(),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	var wg sync.WaitGroup
	func() {
		// setup
		signalCtx, setup, sdown := internal.Setup(appConfig, "kyverno-background-controller", false)
		defer sdown()
		var err error
		bgscanInterval := 30 * time.Second
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
		if err := sanityChecks(setup.ApiServerClient); err != nil {
			setup.Logger.Error(err, "sanity checks failed")
			os.Exit(1)
		}
		// informer factories
		kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)
		polexCache, polexController := internal.NewExceptionSelector(setup.Logger, kyvernoInformer)
		eventGenerator := event.NewEventGenerator(
			setup.EventsClient,
			logging.WithName("EventGenerator"),
			maxQueuedEvents,
			strings.Split(omitEvents, ",")...,
		)
		eventController := internal.NewController(
			event.ControllerName,
			eventGenerator,
			event.Workers,
		)
		urGenerator := generator.NewUpdateRequestGenerator(setup.Configuration, setup.MetadataClient)
		gcstore := store.New()
		gceController := internal.NewController(
			globalcontextcontroller.ControllerName,
			globalcontextcontroller.NewController(
				kyvernoInformer.Kyverno().V2alpha1().GlobalContextEntries(),
				setup.KyvernoDynamicClient,
				setup.KyvernoClient,
				gcstore,
				eventGenerator,
				maxAPICallResponseLength,
				false,
			),
			globalcontextcontroller.Workers,
		) // this controller only subscribe to events, nothing is returned...
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
			polexCache,
			gcstore,
		)
		ephrs, err := breaker.StartBackgroundReportsCounter(signalCtx, setup.MetadataClient)
		if err != nil {
			setup.Logger.Error(err, "failed to start background-scan reports watcher")
			os.Exit(1)
		}
		reportsBreaker := breaker.NewBreaker("background scan reports", func(context.Context) bool {
			count, isRunning := ephrs.Count()
			if !isRunning {
				return true
			}
			return count > maxBackgroundReports
		})
		// start informers and wait for cache sync
		if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
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
				kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, setup.ResyncPeriod)
				kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)
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
					urGenerator,
					setup.ReportingConfiguration,
					reportsBreaker,
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
		// start non leader controllers
		eventController.Run(signalCtx, setup.Logger, &wg)
		gceController.Run(signalCtx, setup.Logger, &wg)
		if polexController != nil {
			polexController.Run(signalCtx, setup.Logger, &wg)
		}
		// start leader election
		le.Run(signalCtx)
	}()
	// wait for everything to shut down and exit
	wg.Wait()
}
