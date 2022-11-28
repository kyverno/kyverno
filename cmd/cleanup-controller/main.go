package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	dynamicclient "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/cleanup"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

var (
	otel                 string
	otelCollector        string
	metricsPort          string
	transportCreds       string
	disableMetricsExport bool
)

const (
	resyncPeriod = 15 * time.Minute
)

func setupMetrics(ctx context.Context, logger logr.Logger, kubeClient kubernetes.Interface) (*metrics.MetricsConfig, context.CancelFunc, error) {
	logger = logger.WithName("metrics")
	logger.Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds)
	metricsConfiguration := internal.GetMetricsConfiguration(logger, kubeClient)
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
		ctx,
		disableMetricsExport,
		otel,
		metricsAddr,
		otelCollector,
		metricsConfiguration,
		transportCreds,
		kubeClient,
		logging.WithName("metrics"),
	)
	if err != nil {
		return nil, nil, err
	}
	var cancel context.CancelFunc
	if otel == "grpc" {
		cancel = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			metrics.ShutDownController(ctx, metricsPusher)
		}
	}
	if otel == "prometheus" {
		go func() {
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				logger.Error(err, "failed to enable metrics", "address", metricsAddr)
			}
		}()
	}
	return metricsConfig, cancel, nil
}

func setupSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func main() {
	// application flags
	flagset := flag.NewFlagSet("application", flag.ExitOnError)
	flagset.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flagset.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flagset.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flagset.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flagset.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
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
	ctx, logger, sdown := internal.Setup()
	defer sdown()
	// create raw client
	rawClient := internal.CreateKubernetesClient(logger)
	// setup metrics
	metricsConfig, metricsShutdown, err := setupMetrics(ctx, logger, rawClient)
	if err != nil {
		logger.Error(err, "failed to setup metrics")
		os.Exit(1)
	}
	if metricsShutdown != nil {
		defer metricsShutdown()
	}
	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	// create instrumented clients
	kubeClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	dynamicClient := internal.CreateDynamicClient(logger, dynamicclient.WithMetrics(metricsConfig, metrics.KyvernoClient), dynamicclient.WithTracing())
	dClient, err := dclient.NewClient(signalCtx, dynamicClient, kubeClient, 15*time.Minute)
	if err != nil {
		logger.Error(err, "failed to create dynamic client")
		os.Exit(1)
	}
	clientConfig := internal.CreateClientConfig(logger)
	kyvernoClient, err := kyvernoclient.NewForConfig(
		clientConfig,
		kyvernoclient.WithMetrics(metricsConfig, metrics.KubeClient),
		kyvernoclient.WithTracing(),
	)
	if err != nil {
		logger.Error(err, "failed to create kyverno client")
		os.Exit(1)
	}
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	cleanupController := cleanup.NewController(
		kubeClient,
		kyvernoInformer.Kyverno().V1alpha1().ClusterCleanupPolicies(),
		kyvernoInformer.Kyverno().V1alpha1().CleanupPolicies(),
		kubeInformer.Batch().V1().CronJobs(),
	)
	controller := newController(cleanup.ControllerName, *cleanupController, cleanup.Workers)
	policyHandlers := NewHandlers(
		dClient,
	)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
	// start informers and wait for cache sync
	// we need to call start again because we potentially registered new informers
	if !internal.StartInformersAndWaitForCacheSync(ctx, kubeKyvernoInformer) {
		os.Exit(1)
	}
	var wg sync.WaitGroup
	controller.run(signalCtx, logger.WithName("cleanup-controller"), &wg)
	server := NewServer(
		policyHandlers,
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Secrets(config.KyvernoNamespace()).Get("cleanup-controller-tls")
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
	)
	// start webhooks server
	server.Run(ctx.Done())
	// wait for termination signal
	<-ctx.Done()
	wg.Wait()
}
