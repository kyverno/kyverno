package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	kubeconfig           string
	clientRateLimitQPS   float64
	clientRateLimitBurst int
	otel                 string
	otelCollector        string
	metricsPort          string
	transportCreds       string
	disableMetricsExport bool
)

const (
	resyncPeriod = 15 * time.Minute
)

func parseFlags(config internal.Configuration) {
	internal.InitFlags(config)
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 20, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 50, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flag.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flag.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
	flag.Parse()
}

func createKubeClients(logger logr.Logger) (*rest.Config, kubernetes.Interface, error) {
	logger = logger.WithName("kube-clients")
	logger.Info("create kube clients...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst)
	if err != nil {
		return nil, nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientConfig, kubeClient, nil
}

func createInstrumentedClients(ctx context.Context, logger logr.Logger, clientConfig *rest.Config, metricsConfig *metrics.MetricsConfig) (kubernetes.Interface, dclient.Interface, error) {
	logger = logger.WithName("instrumented-clients")
	logger.Info("create instrumented clients...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	kubeClient, err := kubeclient.NewForConfig(
		clientConfig,
		kubeclient.WithMetrics(metricsConfig, metrics.KubeClient),
		kubeclient.WithTracing(),
	)
	if err != nil {
		return nil, nil, err
	}
	dynamicClient, err := dclient.NewClient(ctx, clientConfig, kubeClient, metricsConfig, resyncPeriod)
	if err != nil {
		return nil, nil, err
	}
	return kubeClient, dynamicClient, nil
}

func setupMetrics(logger logr.Logger, kubeClient kubernetes.Interface) (*metrics.MetricsConfig, context.CancelFunc, error) {
	logger = logger.WithName("metrics")
	logger.Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds)
	metricsConfigData, err := config.NewMetricsConfigData(kubeClient)
	if err != nil {
		return nil, nil, err
	}
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
		disableMetricsExport,
		otel,
		metricsAddr,
		otelCollector,
		metricsConfigData,
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
	// config
	appConfig := internal.NewConfiguration(internal.WithProfiling(), internal.WithTracing())
	// parse flags
	parseFlags(appConfig)
	// setup logger
	logger := internal.SetupLogger()
	// setup maxprocs
	undo := internal.SetupMaxProcs(logger)
	defer undo()
	// show version
	internal.ShowVersion(logger)
	// start profiling
	internal.SetupProfiling(logger)
	// create client config and kube clients
	clientConfig, rawClient, err := createKubeClients(logger)
	if err != nil {
		os.Exit(1)
	}
	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	// setup metrics
	metricsConfig, metricsShutdown, err := setupMetrics(logger, rawClient)
	if err != nil {
		logger.Error(err, "failed to setup metrics")
		os.Exit(1)
	}
	if metricsShutdown != nil {
		defer metricsShutdown()
	}
	// create instrumented clients
	kubeClient, dynamicClient, err := createInstrumentedClients(signalCtx, logger, clientConfig, metricsConfig)
	if err != nil {
		logger.Error(err, "failed to create instrument clients")
		os.Exit(1)
	}
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	policyHandlers := NewHandlers(
		dynamicClient,
	)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
	// start informers and wait for cache sync
	// we need to call start again because we potentially registered new informers
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, kubeKyvernoInformer) {
		os.Exit(1)
	}
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
	server.Run(signalCtx.Done())
	// wait for termination signal
	<-signalCtx.Done()
}
