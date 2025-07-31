package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"time"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/background"
	"github.com/kyverno/kyverno/pkg/background/gpol"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
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
	"github.com/kyverno/kyverno/pkg/utils/restmapper"
	corev1 "k8s.io/api/core/v1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
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
	context libs.Context,
	gpolEngine gpolengine.Engine,
	gpolProvider gpolengine.Provider,
	mpolEngine mpolengine.Engine,
	mapper meta.RESTMapper,
	reportsConfig reportutils.ReportingConfiguration,
	reportsBreaker breaker.Breaker,
) ([]internal.Controller, error) {
	watchManager := gpol.NewWatchManager(logging.WithName("WatchManager"), dynamicClient)
	policyCtrl, err := policy.NewPolicyController(
		kyvernoClient,
		dynamicClient,
		eng,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Policies().V1alpha1().GeneratingPolicies(),
		kyvernoInformer.Kyverno().V2().UpdateRequests(),
		configuration,
		eventGenerator,
		kubeInformer.Core().V1().Namespaces(),
		logging.WithName("PolicyController"),
		backgroundScanInterval,
		metricsConfig,
		jp,
		urGenerator,
		watchManager,
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
		context,
		gpolEngine,
		gpolProvider,
		watchManager,
		mpolEngine,
		mapper,
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
		genWorkers                      int
		maxQueuedEvents                 int
		omitEvents                      string
		maxAPICallResponseLength        int64
		maxBackgroundReports            int
		controllerRuntimeMetricsAddress string
	)
	flagset := flag.NewFlagSet("updaterequest-controller", flag.ExitOnError)
	flagset.IntVar(&genWorkers, "genWorkers", 10, "Workers for the background controller.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omitEvents", "", "Set this flag to a comma sperated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omitEvents=PolicyApplied,PolicyViolation")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 2*1000*1000, "Maximum allowed response size from API Calls. A value of 0 bypasses checks (not recommended).")
	flagset.IntVar(&maxBackgroundReports, "maxBackgroundReports", 10000, "Maximum number of ephemeralreports created for the background policies.")
	flagset.StringVar(&controllerRuntimeMetricsAddress, "controllerRuntimeMetricsAddress", "", `Bind address for controller-runtime metrics server. It will be defaulted to ":8080" if unspecified. Set this to "0" to disable the metrics server.`)

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
		internal.WithRestConfig(),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	var wg wait.Group
	func() {
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
				setup.KubeClient,
				setup.KyvernoDynamicClient,
				setup.KyvernoClient,
				gcstore,
				eventGenerator,
				maxAPICallResponseLength,
				false,
				setup.Jp,
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
		ephrCounterFunc := func(c breaker.Counter) func(context.Context) bool {
			return func(context.Context) bool {
				count, isRunning := c.Count()
				if !isRunning {
					return true
				}
				return count > maxBackgroundReports
			}
		}
		ephrs, err := breaker.StartAdmissionReportsCounter(signalCtx, setup.MetadataClient)
		if err != nil {
			go func() {
				for {
					ephrs, err := breaker.StartAdmissionReportsCounter(signalCtx, setup.MetadataClient)
					if err != nil {
						setup.Logger.Error(err, "failed to start background scan reports watcher, retrying...")
						time.Sleep(2 * time.Second)
						continue
					}
					breaker.ReportsBreaker = breaker.NewBreaker("background-scan reports", ephrCounterFunc(ephrs))
					return
				}
			}()
			// temporarily create a fake breaker until the retrying goroutine succeeds
			breaker.ReportsBreaker = breaker.NewBreaker("background-scan reports", func(context.Context) bool {
				return true
			})
			// no error occurred, create a normal breaker
		} else {
			breaker.ReportsBreaker = breaker.NewBreaker("background-scan reports", ephrCounterFunc(ephrs))
		}
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
				contextProvider, err := libs.NewContextProvider(
					setup.KyvernoDynamicClient,
					nil,
					gcstore,
					false,
				)
				if err != nil {
					setup.Logger.Error(err, "failed to create cel context provider")
					os.Exit(1)
				}

				namespaceGetter := func(ctx context.Context, name string) *corev1.Namespace {
					ns, err := setup.KubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
					if err != nil {
						return nil
					}
					return ns
				}

				// create compiler
				compiler := gpolcompiler.NewCompiler()
				// create provider
				gpolProvider := gpolengine.NewFetchProvider(
					compiler,
					kyvernoInformer.Policies().V1alpha1().GeneratingPolicies().Lister(),
					kyvernoInformer.Policies().V1alpha1().PolicyExceptions().Lister(),
					internal.PolicyExceptionEnabled(),
				)
				// create engine
				gpolEngine := gpolengine.NewEngine(
					func(name string) *corev1.Namespace {
						return namespaceGetter(signalCtx, name)
					},
					matching.NewMatcher(),
				)

				scheme := kruntime.NewScheme()
				if err := policiesv1alpha1.Install(scheme); err != nil {
					setup.Logger.Error(err, "failed to initialize scheme")
					os.Exit(1)
				}
				mgr, err := ctrl.NewManager(setup.RestConfig, ctrl.Options{
					Scheme: scheme,
					Metrics: server.Options{
						BindAddress: controllerRuntimeMetricsAddress,
					},
				})
				if err != nil {
					setup.Logger.Error(err, "failed to create controller-runtime manager")
					os.Exit(1)
				}

				mgrCtx, mgrCancel := context.WithCancel(signalCtx)
				defer mgrCancel()

				wg.StartWithContext(mgrCtx, func(ctx context.Context) {
					if err := mgr.Start(ctx); err != nil {
						setup.Logger.Error(err, "failed to start manager")
						os.Exit(1)
					}
				})

				if !mgr.GetCache().WaitForCacheSync(mgrCtx) {
					setup.Logger.Error(nil, "failed to sync cache for manager")
					os.Exit(1)
				}

				c := mpolcompiler.NewCompiler()
				mpolProvider, typeConverter, err := mpolengine.NewKubeProvider(mgrCtx, c, mgr, setup.KubeClient.Discovery().OpenAPIV3(), kyvernoInformer.Policies().V1alpha1().PolicyExceptions().Lister(), internal.PolicyExceptionEnabled())
				if err != nil {
					setup.Logger.Error(err, "failed to create mpol provider")
					os.Exit(1)
				}

				mpolEngine := mpolengine.NewEngine(
					mpolProvider,
					func(name string) *corev1.Namespace {
						return namespaceGetter(mgrCtx, name)
					},
					nil,
					typeConverter,
					contextProvider,
				)

				restMapper, err := restmapper.GetRESTMapper(setup.KyvernoDynamicClient, false)
				if err != nil {
					setup.Logger.Error(err, "failed to create RESTMapper")
					os.Exit(1)
				}

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
					contextProvider,
					*gpolEngine,
					gpolProvider,
					mpolEngine,
					restMapper,
					setup.ReportingConfiguration,
					breaker.ReportsBreaker,
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
				var wg wait.Group
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
