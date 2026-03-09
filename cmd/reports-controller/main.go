package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"time"

	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metaclient "github.com/kyverno/kyverno/pkg/clients/metadata"
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
	openreportsclient "github.com/openreports/reports-api/pkg/client/clientset/versioned/typed/openreports.io/v1alpha1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/client-go/discovery/cached/memory"
	kubeinformers "k8s.io/client-go/informers"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1informers "k8s.io/client-go/informers/admissionregistration/v1beta1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/restmapper"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

func sanityChecks(apiserverClient apiserver.Interface, openreportsEnabled bool) error {
	crdNames := []string{
		"ephemeralreports.reports.kyverno.io",
		"clusterephemeralreports.reports.kyverno.io",
	}
	if openreportsEnabled {
		crdNames = append(crdNames, "reports.openreports.io", "clusterreports.openreports.io")
		err := kubeutils.CRDsInstalled(apiserverClient, crdNames...)
		if err != nil {
			return err
		}
		return nil
	}

	crdNames = append(crdNames, "clusterpolicyreports.wgpolicyk8s.io", "policyreports.wgpolicyk8s.io")
	return kubeutils.CRDsInstalled(apiserverClient, crdNames...)
}

func createReportControllers(
	eng engineapi.Engine,
	backgroundScan bool,
	admissionReports bool,
	aggregateReports bool,
	policyReports bool,
	validatingAdmissionPolicyReports bool,
	mutatingAdmissionPolicyReports bool,
	aggregationWorkers int,
	backgroundScanWorkers int,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	orClient openreportsclient.OpenreportsV1alpha1Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	metaClient metaclient.UpstreamInterface,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	backgroundScanInterval time.Duration,
	configuration config.Configuration,
	jp jmespath.Interface,
	eventGenerator event.Interface,
	gcstore store.Store,
	typeConverter patch.TypeConverterManager,
) ([]internal.Controller, func(context.Context) error) {
	var ctrls []internal.Controller
	var warmups []func(context.Context) error
	var vapInformer admissionregistrationv1informers.ValidatingAdmissionPolicyInformer
	var vapBindingInformer admissionregistrationv1informers.ValidatingAdmissionPolicyBindingInformer
	var mapInformer admissionregistrationv1beta1informers.MutatingAdmissionPolicyInformer
	var mapAlphaInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyInformer
	var mapBindingInformer admissionregistrationv1beta1informers.MutatingAdmissionPolicyBindingInformer
	var mapAlphaBindingInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyBindingInformer
	if validatingAdmissionPolicyReports {
		vapInformer = kubeInformer.Admissionregistration().V1().ValidatingAdmissionPolicies()
		vapBindingInformer = kubeInformer.Admissionregistration().V1().ValidatingAdmissionPolicyBindings()
	}
	if mutatingAdmissionPolicyReports {
		mapAlphaInformer = kubeInformer.Admissionregistration().V1alpha1().MutatingAdmissionPolicies()
		mapAlphaBindingInformer = kubeInformer.Admissionregistration().V1alpha1().MutatingAdmissionPolicyBindings()

		if kubeutils.HigherThanKubernetesVersion(client.GetKubeClient().Discovery(), logging.GlobalLogger(), 1, 34, 0) {
			mapInformer = kubeInformer.Admissionregistration().V1beta1().MutatingAdmissionPolicies()
			mapBindingInformer = kubeInformer.Admissionregistration().V1beta1().MutatingAdmissionPolicyBindings()
		}
	}
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV2 := kyvernoInformer.Kyverno().V2()
	policiesV1beta1 := kyvernoInformer.Policies().V1beta1()
	if backgroundScan || admissionReports {
		resourceReportController := resourcereportcontroller.NewController(
			client,
			kyvernoV1.Policies(),
			kyvernoV1.ClusterPolicies(),
			policiesV1beta1.ValidatingPolicies(),
			policiesV1beta1.NamespacedValidatingPolicies(),
			policiesV1beta1.MutatingPolicies(),
			policiesV1beta1.NamespacedMutatingPolicies(),
			policiesV1beta1.ImageValidatingPolicies(),
			policiesV1beta1.NamespacedImageValidatingPolicies(),
			vapInformer,
			mapInformer,
			mapAlphaInformer,
			metaClient,
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
					orClient,
					client,
					metadataFactory,
					kyvernoV1.Policies(),
					kyvernoV1.ClusterPolicies(),
					policiesV1beta1.ValidatingPolicies(),
					policiesV1beta1.NamespacedValidatingPolicies(),
					policiesV1beta1.ImageValidatingPolicies(),
					policiesV1beta1.NamespacedImageValidatingPolicies(),
					policiesV1beta1.GeneratingPolicies(),
					policiesV1beta1.NamespacedGeneratingPolicies(),
					policiesV1beta1.MutatingPolicies(),
					policiesV1beta1.NamespacedMutatingPolicies(),
					vapInformer,
					mapInformer,
					mapAlphaInformer,
				),
				aggregationWorkers,
			))
		}
		if backgroundScan {
			restMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(client.GetKubeClient().Discovery()))

			backgroundScanController := backgroundscancontroller.NewController(
				client,
				kyvernoClient,
				eng,
				metadataFactory,
				kyvernoV1.Policies(),
				kyvernoV1.ClusterPolicies(),
				policiesV1beta1.ValidatingPolicies(),
				policiesV1beta1.NamespacedValidatingPolicies(),
				policiesV1beta1.MutatingPolicies(),
				policiesV1beta1.NamespacedMutatingPolicies(),
				policiesV1beta1.ImageValidatingPolicies(),
				policiesV1beta1.NamespacedImageValidatingPolicies(),
				policiesV1beta1.PolicyExceptions(),
				kyvernoV2.PolicyExceptions(),
				vapInformer,
				vapBindingInformer,
				mapInformer,
				mapAlphaInformer,
				mapBindingInformer,
				mapAlphaBindingInformer,
				kubeInformer.Core().V1().Namespaces(),
				resourceReportController,
				backgroundScanInterval,
				configuration,
				jp,
				eventGenerator,
				policyReports,
				gcstore,
				restMapper,
				typeConverter,
			)
			ctrls = append(ctrls, internal.NewController(
				backgroundscancontroller.ControllerName,
				backgroundScanController,
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
	reportsConfig reportutils.ReportingConfiguration,
	aggregateReports bool,
	policyReports bool,
	validatingAdmissionPolicyReports bool,
	mutatingAdmissionPolicyReports bool,
	aggregationWorkers int,
	backgroundScanWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	metadataInformer metadatainformers.SharedInformerFactory,
	metaClient metaclient.UpstreamInterface,
	kyvernoClient versioned.Interface,
	orClient openreportsclient.OpenreportsV1alpha1Interface,
	dynamicClient dclient.Interface,
	configuration config.Configuration,
	jp jmespath.Interface,
	eventGenerator event.Interface,
	backgroundScanInterval time.Duration,
	gcstore store.Store,
	typeConverter patch.TypeConverterManager,
) ([]internal.Controller, func(context.Context) error, error) {
	reportControllers, warmup := createReportControllers(
		eng,
		backgroundScan,
		admissionReports,
		aggregateReports,
		policyReports,
		validatingAdmissionPolicyReports,
		mutatingAdmissionPolicyReports,
		aggregationWorkers,
		backgroundScanWorkers,
		dynamicClient,
		kyvernoClient,
		orClient,
		metadataInformer,
		metaClient,
		kubeInformer,
		kyvernoInformer,
		backgroundScanInterval,
		configuration,
		jp,
		eventGenerator,
		gcstore,
		typeConverter,
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
		mutatingAdmissionPolicyReports   bool
		reportsCRDsSanityChecks          bool
		backgroundScanWorkers            int
		backgroundScanInterval           time.Duration
		aggregationWorkers               int
		maxQueuedEvents                  int
		omitEvents                       string
		skipResourceFilters              bool
		maxAPICallResponseLength         int64
		apiCallTimeout                   time.Duration
		maxBackgroundReports             int
	)
	flagset := flag.NewFlagSet("reports-controller", flag.ExitOnError)
	flagset.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable background scan.")
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.BoolVar(&aggregateReports, "aggregateReports", true, "Enable or disable aggregated policy reports.")
	flagset.BoolVar(&policyReports, "policyReports", true, "Enable or disable policy reports.")
	flagset.BoolVar(&validatingAdmissionPolicyReports, "validatingAdmissionPolicyReports", true, "Enable or disable ValidatingAdmissionPolicy reports.")
	flagset.BoolVar(&mutatingAdmissionPolicyReports, "mutatingAdmissionPolicyReports", false, "Enable or disable MutatingAdmissionPolicy reports.")
	flagset.IntVar(&aggregationWorkers, "aggregationWorkers", aggregatereportcontroller.Workers, "Configure the number of ephemeral reports aggregation workers.")
	flagset.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	flagset.DurationVar(&backgroundScanInterval, "backgroundScanInterval", time.Hour, "Configure background scan interval.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omitEvents", "", "Set this flag to a comma separated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omitEvents=PolicyApplied,PolicyViolation")
	flagset.BoolVar(&skipResourceFilters, "skipResourceFilters", true, "If true, resource filters wont be considered.")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 2*1000*1000, "Maximum allowed response size from API Calls. A value of 0 bypasses checks (not recommended).")
	flagset.DurationVar(&apiCallTimeout, "apiCallTimeout", 30*time.Second, "Timeout for HTTP API calls made by policies. A value of 0 means no timeout.")
	flagset.IntVar(&maxBackgroundReports, "maxBackgroundReports", 10000, "Maximum number of ephemeralreports created for the background policies before we stop creating new ones")
	flagset.BoolVar(&reportsCRDsSanityChecks, "reportsCRDsSanityChecks", true, "Enable or disable sanity checks for policy reports and ephemeral reports CRDs.")
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
		internal.WithOpenreports(),
		internal.WithMetadataClient(),
	)
	// parse flags
	internal.ParseFlags(
		appConfig,
		internal.WithDefaultQps(300),
		internal.WithDefaultBurst(300),
	)
	var wg wait.Group
	func() {
		// setup
		ctx, setup, sdown := internal.Setup(appConfig, "kyverno-reports-controller", skipResourceFilters)
		defer sdown()
		// THIS IS AN UGLY FIX
		// ELSE KYAML IS NOT THREAD SAFE
		kyamlopenapi.Schema()
		if err := sanityChecks(setup.ApiServerClient, setup.OpenreportsClient != nil); err != nil {
			setup.Logger.Error(err, "sanity checks failed")
			if reportsCRDsSanityChecks {
				os.Exit(1)
			}
		}
		if mutatingAdmissionPolicyReports {
			registered, err := admissionpolicy.IsMutatingAdmissionPolicyRegistered(setup.KubeClient)
			if !registered {
				setup.Logger.Error(err, "MutatingAdmissionPolicies isn't supported in the API server")
				os.Exit(1)
			}
		}
		setup.Logger.V(2).Info("background scan interval", "duration", backgroundScanInterval.String())

		// call NewContextProvider to initialize the libraries context globally, needed during background scan
		gcstore := store.New()
		restMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(setup.KubeClient.Discovery()))
		_, err := libs.NewContextProvider(
			setup.KyvernoDynamicClient,
			nil,
			gcstore,
			restMapper,
			false,
		)
		if err != nil {
			setup.Logger.Error(err, "failed to create cel context provider")
			os.Exit(1)
		}
		// informer factories
		kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)
		polexCache, polexController := internal.NewExceptionSelector(setup.Logger, kyvernoInformer)
		eventGenerator := event.NewEventGenerator(
			setup.EventsClient,
			logging.WithName("EventGenerator"),
			maxQueuedEvents,
			setup.Configuration,
			strings.Split(omitEvents, ",")...,
		)
		eventController := internal.NewController(
			event.ControllerName,
			eventGenerator,
			event.Workers,
		)
		gceController := internal.NewController(
			globalcontextcontroller.ControllerName,
			globalcontextcontroller.NewController(
				kyvernoInformer.Kyverno().V2beta1().GlobalContextEntries(),
				setup.KubeClient,
				setup.KyvernoDynamicClient,
				setup.KyvernoClient,
				gcstore,
				eventGenerator,
				maxAPICallResponseLength,
				apiCallTimeout,
				false,
				setup.Jp,
			),
			globalcontextcontroller.Workers,
		)
		// engine
		engine := internal.NewEngine(
			ctx,
			setup.Logger,
			setup.Configuration,
			setup.Jp,
			setup.KyvernoDynamicClient,
			setup.RegistryClient,
			setup.ImageVerifyCacheClient,
			setup.KubeClient,
			setup.KyvernoClient,
			setup.RegistrySecretLister,
			apicall.NewAPICallConfiguration(maxAPICallResponseLength, apiCallTimeout),
			polexCache,
			gcstore,
		)
		// start informers and wait for cache sync
		if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kyvernoInformer) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
		ephrCounterFunc := func(c breaker.Counter) func(context.Context) bool {
			return func(context.Context) bool {
				count, isRunning := c.Count()
				if !isRunning {
					return true
				}
				return count > maxBackgroundReports
			}
		}
		ephrs, err := breaker.StartBackgroundReportsCounter(ctx, setup.MetadataClient)
		if err != nil {
			go func() {
				for {
					ephrs, err := breaker.StartBackgroundReportsCounter(ctx, setup.MetadataClient)
					if err != nil {
						setup.Logger.Error(err, "failed to start background scan reports watcher, retrying...")
						time.Sleep(2 * time.Second)
						continue
					}
					breaker.SetReportsBreaker(breaker.NewBreaker("background scan reports", ephrCounterFunc(ephrs)))
					return
				}
			}()
			// create a temporary breaker until the retrying goroutine succeeds
			breaker.SetReportsBreaker(breaker.NewBreaker("background scan reports", func(context.Context) bool {
				return true
			}))
			// no error occurred, create a normal breaker
		} else {
			breaker.SetReportsBreaker(breaker.NewBreaker("background scan reports", ephrCounterFunc(ephrs)))
		}

		typeConverter := patch.NewTypeConverterManager(nil, setup.KubeClient.Discovery().OpenAPIV3())
		go typeConverter.Run(ctx)

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
					mutatingAdmissionPolicyReports,
					aggregationWorkers,
					backgroundScanWorkers,
					kubeInformer,
					kyvernoInformer,
					metadataInformer,
					setup.MetadataClient,
					setup.KyvernoClient,
					setup.OpenreportsClient,
					setup.KyvernoDynamicClient,
					setup.Configuration,
					setup.Jp,
					eventGenerator,
					backgroundScanInterval,
					gcstore,
					typeConverter,
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
				var wg wait.Group
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
