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
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	admissionreportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/admission"
	aggregatereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	backgroundscancontroller "github.com/kyverno/kyverno/pkg/controllers/report/background"
	resourcereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

const (
	resyncPeriod = 15 * time.Minute
)

func createReportControllers(
	eng engineapi.Engine,
	backgroundScan bool,
	admissionReports bool,
	reportsChunkSize int,
	backgroundScanWorkers int,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	rclient registryclient.Client,
	metadataFactory metadatainformers.SharedInformerFactory,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	backgroundScanInterval time.Duration,
	configuration config.Configuration,
	jp jmespath.Interface,
	eventGenerator event.Interface,
) ([]internal.Controller, func(context.Context) error) {
	var ctrls []internal.Controller
	var warmups []func(context.Context) error
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	if backgroundScan || admissionReports {
		resourceReportController := resourcereportcontroller.NewController(
			client,
			kyvernoV1.Policies(),
			kyvernoV1.ClusterPolicies(),
		)
		warmups = append(warmups, func(ctx context.Context) error {
			return resourceReportController.Warmup(ctx)
		})
		ctrls = append(ctrls, internal.NewController(
			resourcereportcontroller.ControllerName,
			resourceReportController,
			resourcereportcontroller.Workers,
		))
		ctrls = append(ctrls, internal.NewController(
			aggregatereportcontroller.ControllerName,
			aggregatereportcontroller.NewController(
				kyvernoClient,
				metadataFactory,
				kyvernoV1.Policies(),
				kyvernoV1.ClusterPolicies(),
				resourceReportController,
				reportsChunkSize,
			),
			aggregatereportcontroller.Workers,
		))
		if admissionReports {
			ctrls = append(ctrls, internal.NewController(
				admissionreportcontroller.ControllerName,
				admissionreportcontroller.NewController(
					kyvernoClient,
					client,
					metadataFactory,
				),
				admissionreportcontroller.Workers,
			))
		}
		if backgroundScan {
			ctrls = append(ctrls, internal.NewController(
				backgroundscancontroller.ControllerName,
				backgroundscancontroller.NewController(
					client,
					kyvernoClient,
					eng,
					metadataFactory,
					kyvernoV1.Policies(),
					kyvernoV1.ClusterPolicies(),
					kubeInformer.Core().V1().Namespaces(),
					resourceReportController,
					backgroundScanInterval,
					configuration,
					jp,
					eventGenerator,
				),
				backgroundScanWorkers,
			))
		}
	}
	return ctrls, func(ctx context.Context) error {
		for _, warmup := range warmups {
			if err := warmup(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

func createrLeaderControllers(
	eng engineapi.Engine,
	backgroundScan bool,
	admissionReports bool,
	reportsChunkSize int,
	backgroundScanWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	metadataInformer metadatainformers.SharedInformerFactory,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	rclient registryclient.Client,
	configuration config.Configuration,
	jp jmespath.Interface,
	eventGenerator event.Interface,
	backgroundScanInterval time.Duration,
) ([]internal.Controller, func(context.Context) error, error) {
	reportControllers, warmup := createReportControllers(
		eng,
		backgroundScan,
		admissionReports,
		reportsChunkSize,
		backgroundScanWorkers,
		dynamicClient,
		kyvernoClient,
		rclient,
		metadataInformer,
		kubeInformer,
		kyvernoInformer,
		backgroundScanInterval,
		configuration,
		jp,
		eventGenerator,
	)
	return reportControllers, warmup, nil
}

func main() {
	var (
		backgroundScan         bool
		admissionReports       bool
		reportsChunkSize       int
		backgroundScanWorkers  int
		backgroundScanInterval time.Duration
		maxQueuedEvents        int
		omitEvents             string
		skipResourceFilters    bool
	)
	flagset := flag.NewFlagSet("reports-controller", flag.ExitOnError)
	flagset.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable backgound scan.")
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.IntVar(&reportsChunkSize, "reportsChunkSize", 1000, "Max number of results in generated reports, reports will be split accordingly if there are more results to be stored.")
	flagset.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	flagset.DurationVar(&backgroundScanInterval, "backgroundScanInterval", time.Hour, "Configure background scan interval.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omit-events", "", "Set this flag to a comma sperated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omit-events=PolicyApplied,PolicyViolation")
	flagset.BoolVar(&skipResourceFilters, "skipResourceFilters", true, "If true, resource filters wont be considered.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithPolicyExceptions(),
		internal.WithConfigMapCaching(),
		internal.WithCosign(),
		internal.WithRegistryClient(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithDynamicClient(),
		internal.WithMetadataClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(
		appConfig,
		internal.WithDefaultQps(300),
		internal.WithDefaultBurst(300),
	)
	// setup
	ctx, setup, sdown := internal.Setup(appConfig, "kyverno-reports-controller", skipResourceFilters)
	defer sdown()
	// THIS IS AN UGLY FIX
	// ELSE KYAML IS NOT THREAD SAFE
	kyamlopenapi.Schema()
	setup.Logger.Info("background scan interval", "duration", backgroundScanInterval.String())
	// informer factories
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
	omitEventsValues := strings.Split(omitEvents, ",")
	if omitEvents == "" {
		omitEventsValues = []string{}
	}
	eventGenerator := event.NewEventGenerator(
		setup.KyvernoDynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		omitEventsValues,
		logging.WithName("EventGenerator"),
	)
	// engine
	engine := internal.NewEngine(
		ctx,
		setup.Logger,
		setup.Configuration,
		setup.MetricsConfiguration,
		setup.Jp,
		setup.KyvernoDynamicClient,
		setup.RegistryClient,
		setup.KubeClient,
		setup.KyvernoClient,
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kyvernoInformer) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// start event generator
	var wg sync.WaitGroup
	go eventGenerator.Run(ctx, 3, &wg)
	// setup leader election
	le, err := leaderelection.New(
		setup.Logger.WithName("leader-election"),
		"kyverno-reports-controller",
		config.KyvernoNamespace(),
		setup.LeaderElectionClient,
		config.KyvernoPodName(),
		internal.LeaderElectionRetryPeriod(),
		func(ctx context.Context) {
			logger := setup.Logger.WithName("leader")
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, resyncPeriod)
			kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
			metadataInformer := metadatainformers.NewSharedInformerFactory(setup.MetadataClient, 15*time.Minute)
			// create leader controllers
			leaderControllers, warmup, err := createrLeaderControllers(
				engine,
				backgroundScan,
				admissionReports,
				reportsChunkSize,
				backgroundScanWorkers,
				kubeInformer,
				kyvernoInformer,
				metadataInformer,
				setup.KyvernoClient,
				setup.KyvernoDynamicClient,
				setup.RegistryClient,
				setup.Configuration,
				setup.Jp,
				eventGenerator,
				backgroundScanInterval,
			)
			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(ctx, logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			internal.StartInformers(ctx, metadataInformer)
			if !internal.CheckCacheSync(logger, metadataInformer.WaitForCacheSync(ctx.Done())) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			if err := warmup(ctx); err != nil {
				logger.Error(err, "failed to run warmup")
				os.Exit(1)
			}
			// start leader controllers
			var wg sync.WaitGroup
			for _, controller := range leaderControllers {
				controller.Run(ctx, logger.WithName("controllers"), &wg)
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
	le.Run(ctx)
	sdown()
	wg.Wait()
}
