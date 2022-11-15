package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/wrappers"
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
	logFormat            string
	otel                 string
	otelCollector        string
	metricsPort          string
	transportCreds       string
	disableMetricsExport bool
)

const (
	resyncPeriod         = 15 * time.Minute
	metadataResyncPeriod = 15 * time.Minute
)

func parseFlags() error {
	logging.Init(nil)
	flag.StringVar(&logFormat, "loggingFormat", logging.TextFormat, "This determines the output format of the logger.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 20, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 50, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flag.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flag.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
	if err := flag.Set("v", "2"); err != nil {
		return err
	}
	flag.Parse()
	return nil
}

func createKubeClients(logger logr.Logger) (*rest.Config, *kubernetes.Clientset, error) {
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

func createInstrumentedClients(ctx context.Context, logger logr.Logger, clientConfig *rest.Config, kubeClient *kubernetes.Clientset, metricsConfig *metrics.MetricsConfig) (versioned.Interface, dclient.Interface, error) {
	logger = logger.WithName("instrumented-clients")
	logger.Info("create instrumented clients...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	kyvernoClient, err := kyvernoclient.NewForConfig(clientConfig, metricsConfig)
	if err != nil {
		return nil, nil, err
	}
	dynamicClient, err := dclient.NewClient(ctx, clientConfig, kubeClient, metricsConfig, metadataResyncPeriod)
	if err != nil {
		return nil, nil, err
	}
	return kyvernoClient, dynamicClient, nil
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
	// parse flags
	if err := parseFlags(); err != nil {
		fmt.Println("failed to parse flags", err)
		os.Exit(1)
	}
	// setup logger
	logLevel, err := strconv.Atoi(flag.Lookup("v").Value.String())
	if err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	if err := logging.Setup(logFormat, logLevel); err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	logger := logging.WithName("setup")
	// create client config and kube clients
	clientConfig, kubeClient, err := createKubeClients(logger)
	if err != nil {
		os.Exit(1)
	}
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	metricsConfig, metricsShutdown, err := setupMetrics(logger, kubeClient)
	if err != nil {
		logger.Error(err, "failed to setup metrics")
		os.Exit(1)
	}
	if metricsShutdown != nil {
		defer metricsShutdown()
	}
	_, dynamicClient, err := createInstrumentedClients(signalCtx, logger, clientConfig, kubeClient, metricsConfig)
	if err != nil {
		logger.Error(err, "failed to create instrument clients")
		os.Exit(1)
	}
	policyHandlers := NewHandlers(
		dynamicClient,
	)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
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
	// start informers and wait for cache sync
	// we need to call start again because we potentially registered new informers
	if !startInformersAndWaitForCacheSync(signalCtx, kubeKyvernoInformer) {
		os.Exit(1)
	}
	// start webhooks server
	server.Run(signalCtx.Done())
	// wait for termination signal
	<-signalCtx.Done()
}
