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
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	dynamicclient "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	metadataclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/config"
	admissionreportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/admission"
	aggregatereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	backgroundscancontroller "github.com/kyverno/kyverno/pkg/controllers/report/background"
	resourcereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
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

func createReportControllers(
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
	configMapResolver resolvers.ConfigmapResolver,
	backgroundScanInterval time.Duration,
	configuration config.Configuration,
	eventGenerator event.Interface,
) ([]internal.Controller, func(context.Context) error) {
	var ctrls []internal.Controller
	var warmups []func(context.Context) error
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV2Alpha1 := kyvernoInformer.Kyverno().V2alpha1()
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
					metadataFactory,
					resourceReportController,
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
					rclient,
					metadataFactory,
					kyvernoV1.Policies(),
					kyvernoV1.ClusterPolicies(),
					kubeInformer.Core().V1().Namespaces(),
					kyvernoV2Alpha1.PolicyExceptions(),
					resourceReportController,
					configMapResolver,
					backgroundScanInterval,
					configuration,
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
	eventGenerator event.Interface,
	configMapResolver resolvers.ConfigmapResolver,
	backgroundScanInterval time.Duration,
) ([]internal.Controller, func(context.Context) error, error) {
	reportControllers, warmup := createReportControllers(
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
		configMapResolver,
		backgroundScanInterval,
		configuration,
		eventGenerator,
	)
	return reportControllers, warmup, nil
}

func main() {
	var (
		leaderElectionRetryPeriod time.Duration
		imagePullSecrets          string
		imageSignatureRepository  string
		allowInsecureRegistry     bool
		backgroundScan            bool
		admissionReports          bool
		reportsChunkSize          int
		backgroundScanWorkers     int
		backgroundScanInterval    time.Duration
		maxQueuedEvents           int
	)
	flagset := flag.NewFlagSet("reports-controller", flag.ExitOnError)
	flagset.DurationVar(&leaderElectionRetryPeriod, "leaderElectionRetryPeriod", leaderelection.DefaultRetryPeriod, "Configure leader election retry period.")
	flagset.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flagset.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flagset.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flagset.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable backgound scan.")
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.IntVar(&reportsChunkSize, "reportsChunkSize", 1000, "Max number of results in generated reports, reports will be split accordingly if there are more results to be stored.")
	flagset.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	flagset.DurationVar(&backgroundScanInterval, "backgroundScanInterval", time.Hour, "Configure background scan interval.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
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
	ctx, logger, metricsConfig, sdown := internal.Setup()
	defer sdown()
	// create instrumented clients
	kubeClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	leaderElectionClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	kyvernoClient := internal.CreateKyvernoClient(logger, kyvernoclient.WithMetrics(metricsConfig, metrics.KyvernoClient), kyvernoclient.WithTracing())
	metadataClient := internal.CreateMetadataClient(logger, metadataclient.WithMetrics(metricsConfig, metrics.KyvernoClient), metadataclient.WithTracing())
	dynamicClient := internal.CreateDynamicClient(logger, dynamicclient.WithMetrics(metricsConfig, metrics.KyvernoClient), dynamicclient.WithTracing())
	dClient, err := dclient.NewClient(ctx, dynamicClient, kubeClient, 15*time.Minute)
	if err != nil {
		logger.Error(err, "failed to create dynamic client")
		os.Exit(1)
	}
	// THIS IS AN UGLY FIX
	// ELSE KYAML IS NOT THREAD SAFE
	kyamlopenapi.Schema()
	// informer factories
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	cacheInformer, err := resolvers.GetCacheInformerFactory(kubeClient, resyncPeriod)
	if err != nil {
		logger.Error(err, "failed to create cache informer factory")
		os.Exit(1)
	}
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
	// setup registry client
	rclient, err := setupRegistryClient(ctx, logger, secretLister, imagePullSecrets, allowInsecureRegistry)
	if err != nil {
		logger.Error(err, "failed to setup registry client")
		os.Exit(1)
	}
	// setup cosign
	setupCosign(logger, imageSignatureRepository)
	informerBasedResolver, err := resolvers.NewInformerBasedResolver(cacheInformer.Core().V1().ConfigMaps().Lister())
	if err != nil {
		logger.Error(err, "failed to create informer based resolver")
		os.Exit(1)
	}
	clientBasedResolver, err := resolvers.NewClientBasedResolver(kubeClient)
	if err != nil {
		logger.Error(err, "failed to create client based resolver")
		os.Exit(1)
	}
	configMapResolver, err := resolvers.NewResolverChain(informerBasedResolver, clientBasedResolver)
	if err != nil {
		logger.Error(err, "failed to create config map resolver")
		os.Exit(1)
	}
	configuration, err := config.NewConfiguration(kubeClient)
	if err != nil {
		logger.Error(err, "failed to initialize configuration")
		os.Exit(1)
	}
	eventGenerator := event.NewEventGenerator(
		dClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		logging.WithName("EventGenerator"),
	)
	// setup leader election
	le, err := leaderelection.New(
		logger.WithName("leader-election"),
		"kyverno-reports-controller",
		config.KyvernoNamespace(),
		leaderElectionClient,
		config.KyvernoPodName(),
		leaderElectionRetryPeriod,
		func(ctx context.Context) {
			logger := logger.WithName("leader")
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
			kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
			metadataInformer := metadatainformers.NewSharedInformerFactory(metadataClient, 15*time.Minute)
			// create leader controllers
			leaderControllers, warmup, err := createrLeaderControllers(
				backgroundScan,
				admissionReports,
				reportsChunkSize,
				backgroundScanWorkers,
				kubeInformer,
				kyvernoInformer,
				metadataInformer,
				kyvernoClient,
				dClient,
				rclient,
				configuration,
				eventGenerator,
				configMapResolver,
				backgroundScanInterval,
			)
			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(ctx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			internal.StartInformers(ctx, metadataInformer)
			if !internal.CheckCacheSync(metadataInformer.WaitForCacheSync(ctx.Done())) {
				// TODO: shall we just exit ?
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
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
		logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			le.Run(ctx)
		}
	}
}
