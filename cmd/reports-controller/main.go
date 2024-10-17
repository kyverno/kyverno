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
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	globalcontextcontroller "github.com/kyverno/kyverno/pkg/controllers/globalcontext"
	aggregatereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	backgroundscancontroller "github.com/kyverno/kyverno/pkg/controllers/report/background"
	resourcereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/client-go/informers"
	admissionregistrationv1beta1informers "k8s.io/client-go/informers/admissionregistration/v1beta1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

func sanityChecks(apiserverClient apiserver.Interface) error {
	return kubeutils.CRDsInstalled(apiserverClient,
		"clusterpolicyreports.wgpolicyk8s.io",
		"policyreports.wgpolicyk8s.io",
		"ephemeralreports.reports.kyverno.io",
		"clusterephemeralreports.reports.kyverno.io",
	)
}

func createReportControllers(
	eng engineapi.Engine,
	backgroundScan bool,
	admissionReports bool,
	aggregateReports bool,
	policyReports bool,
	validatingAdmissionPolicyReports bool,
	aggregationWorkers int,
	backgroundScanWorkers int,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	backgroundScanInterval time.Duration,
	configuration config.Configuration,
	jp jmespath.Interface,
	eventGenerator event.Interface,
	reportsConfig reportutils.ReportingConfiguration,
	reportsBreaker breaker.Breaker,
) ([]internal.Controller, func(context.Context) error) {
	var ctrls []internal.Controller
	var warmups []func(context.Context) error
	var vapInformer admissionregistrationv1beta1informers.ValidatingAdmissionPolicyInformer
	var vapBindingInformer admissionregistrationv1beta1informers.ValidatingAdmissionPolicyBindingInformer
	// check if validating admission policies are registered in the API server
	if validatingAdmissionPolicyReports {
		vapInformer = kubeInformer.Admissionregistration().V1beta1().ValidatingAdmissionPolicies()
		vapBindingInformer = kubeInformer.Admissionregistration().V1beta1().ValidatingAdmissionPolicyBindings()
	}
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV2 := kyvernoInformer.Kyverno().V2()
	if backgroundScan || admissionReports {
		resourceReportController := resourcereportcontroller.NewController(
			client,
			kyvernoV1.Policies(),
			kyvernoV1.ClusterPolicies(),
			vapInformer,
		)
		warmups = append(warmups, func(ctx context.Context) error {
			return resourceReportController.Warmup(ctx)
		})
		ctrls = append(ctrls, internal.NewController(
			resourcereportcontroller.ControllerName,
			resourceReportController,
			resourcereportcontroller.Workers,
		))
		if aggregateReports {
			ctrls = append(ctrls, internal.NewController(
				aggregatereportcontroller.ControllerName,
				aggregatereportcontroller.NewController(
					kyvernoClient,
					client,
					metadataFactory,
					kyvernoV1.Policies(),
					kyvernoV1.ClusterPolicies(),
					vapInformer,
				),
				aggregationWorkers,
			))
		}
		if backgroundScan {
			backgroundScanController := backgroundscancontroller.NewController(
				client,
				kyvernoClient,
				eng,
				metadataFactory,
				kyvernoV1.Policies(),
				kyvernoV1.ClusterPolicies(),
				kyvernoV2.PolicyExceptions(),
				vapInformer,
				vapBindingInformer,
				kubeInformer.Core().V1().Namespaces(),
				resourceReportController,
				backgroundScanInterval,
				configuration,
				jp,
				eventGenerator,
				policyReports,
				reportsConfig,
				reportsBreaker,
			)
			ctrls = append(ctrls, internal.NewController(
				backgroundscancontroller.ControllerName,
				backgroundScanController,
				backgroundScanWorkers),
			)
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
	reportsConfig reportutils.ReportingConfiguration,
	aggregateReports bool,
	policyReports bool,
	validatingAdmissionPolicyReports bool,
	aggregationWorkers int,
	backgroundScanWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	metadataInformer metadatainformers.SharedInformerFactory,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	configuration config.Configuration,
	jp jmespath.Interface,
	eventGenerator event.Interface,
	backgroundScanInterval time.Duration,
	reportsBreaker breaker.Breaker,
) ([]internal.Controller, func(context.Context) error, error) {
	reportControllers, warmup := createReportControllers(
		eng,
		backgroundScan,
		admissionReports,
		aggregateReports,
		policyReports,
		validatingAdmissionPolicyReports,
		aggregationWorkers,
		backgroundScanWorkers,
		dynamicClient,
		kyvernoClient,
		metadataInformer,
		kubeInformer,
		kyvernoInformer,
		backgroundScanInterval,
		configuration,
		jp,
		eventGenerator,
		reportsConfig,
		reportsBreaker,
	)
	return reportControllers, warmup, nil
}

func main() {
	var (
		backgroundScan                   bool
		admissionReports                 bool
		aggregateReports                 bool
		policyReports                    bool
		validatingAdmissionPolicyReports bool
		backgroundScanWorkers            int
		backgroundScanInterval           time.Duration
		aggregationWorkers               int
		maxQueuedEvents                  int
		omitEvents                       string
		skipResourceFilters              bool
		maxAPICallResponseLength         int64
		maxBackgroundReports             int
	)
	flagset := flag.NewFlagSet("reports-controller", flag.ExitOnError)
	flagset.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable background scan.")
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.BoolVar(&aggregateReports, "aggregateReports", true, "Enable or disable aggregated policy reports.")
	flagset.BoolVar(&policyReports, "policyReports", true, "Enable or disable policy reports.")
	flagset.BoolVar(&validatingAdmissionPolicyReports, "validatingAdmissionPolicyReports", false, "Enable or disable validating admission policy reports.")
	flagset.IntVar(&aggregationWorkers, "aggregationWorkers", aggregatereportcontroller.Workers, "Configure the number of ephemeral reports aggregation workers.")
	flagset.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	flagset.DurationVar(&backgroundScanInterval, "backgroundScanInterval", time.Hour, "Configure background scan interval.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omitEvents", "", "Set this flag to a comma separated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omitEvents=PolicyApplied,PolicyViolation")
	flagset.BoolVar(&skipResourceFilters, "skipResourceFilters", true, "If true, resource filters wont be considered.")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 2*1000*1000, "Maximum allowed response size from API Calls. A value of 0 bypasses checks (not recommended).")
	flagset.IntVar(&maxBackgroundReports, "maxBackgroundReports", 10000, "Maximum number of ephemeralreports created for the background policies before we stop creating new ones")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithPolicyExceptions(),
		internal.WithConfigMapCaching(),
		internal.WithDeferredLoading(),
		internal.WithCosign(),
		internal.WithRegistryClient(),
		internal.WithImageVerifyCache(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithDynamicClient(),
		internal.WithMetadataClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithEventsClient(),
		internal.WithApiServerClient(),
		internal.WithFlagSets(flagset),
		internal.WithReporting(),
	)
	// parse flags
	internal.ParseFlags(
		appConfig,
		internal.WithDefaultQps(300),
		internal.WithDefaultBurst(300),
	)
	var wg sync.WaitGroup
	func() {
		// setup
		ctx, setup, sdown := internal.Setup(appConfig, "kyverno-reports-controller", skipResourceFilters)
		defer sdown()
		// THIS IS AN UGLY FIX
		// ELSE KYAML IS NOT THREAD SAFE
		kyamlopenapi.Schema()
		if err := sanityChecks(setup.ApiServerClient); err != nil {
			setup.Logger.Error(err, "sanity checks failed")
			os.Exit(1)
		}
		setup.Logger.Info("background scan interval", "duration", backgroundScanInterval.String())
		// check if validating admission policies are registered in the API server
		if validatingAdmissionPolicyReports {
			registered, err := validatingadmissionpolicy.IsValidatingAdmissionPolicyRegistered(setup.KubeClient)
			if !registered {
				setup.Logger.Error(err, "ValidatingAdmissionPolicies isn't supported in the API server")
				os.Exit(1)
			}
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
			setup.ImageVerifyCacheClient,
			setup.KubeClient,
			setup.KyvernoClient,
			setup.RegistrySecretLister,
			apicall.NewAPICallConfiguration(maxAPICallResponseLength),
			polexCache,
			gcstore,
		)
		// start informers and wait for cache sync
		if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kyvernoInformer) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
		ephrs, err := breaker.StartBackgroundReportsCounter(ctx, setup.MetadataClient)
		if err != nil {
			setup.Logger.Error(err, "failed to start background-scan reports watcher")
			os.Exit(1)
		}

		// create the circuit breaker
		reportsBreaker := breaker.NewBreaker("background scan reports", func(context.Context) bool {
			count, isRunning := ephrs.Count()
			if !isRunning {
				return true
			}
			return count > maxBackgroundReports
		})
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
				kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, setup.ResyncPeriod)
				kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, setup.ResyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
				kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)
				metadataInformer := metadatainformers.NewSharedInformerFactory(setup.MetadataClient, setup.ResyncPeriod)
				// create leader controllers
				leaderControllers, warmup, err := createrLeaderControllers(
					engine,
					backgroundScan,
					admissionReports,
					setup.ReportingConfiguration,
					aggregateReports,
					policyReports,
					validatingAdmissionPolicyReports,
					aggregationWorkers,
					backgroundScanWorkers,
					kubeInformer,
					kyvernoInformer,
					metadataInformer,
					setup.KyvernoClient,
					setup.KyvernoDynamicClient,
					setup.Configuration,
					setup.Jp,
					eventGenerator,
					backgroundScanInterval,
					reportsBreaker,
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
		// start non leader controllers
		eventController.Run(ctx, setup.Logger, &wg)
		gceController.Run(ctx, setup.Logger, &wg)
		if polexController != nil {
			polexController.Run(ctx, setup.Logger, &wg)
		}
		// start leader election
		le.Run(ctx)
	}()
	// wait for everything to shut down and exit
	wg.Wait()
}
