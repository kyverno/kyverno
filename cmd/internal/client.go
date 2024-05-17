package internal

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	agg "github.com/kyverno/kyverno/pkg/clients/aggregator"
	apisrv "github.com/kyverno/kyverno/pkg/clients/apiserver"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	dyn "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyverno "github.com/kyverno/kyverno/pkg/clients/kyverno"
	meta "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

func createClientConfig(logger logr.Logger, rateLimitQPS float64, rateLimitBurst int) *rest.Config {
	clientConfig, err := config.CreateClientConfig(kubeconfig, rateLimitQPS, rateLimitBurst)
	checkError(logger, err, "failed to create rest client configuration")
	clientConfig.Wrap(
		func(base http.RoundTripper) http.RoundTripper {
			return tracing.Transport(base, otelhttp.WithFilter(tracing.RequestFilterIsInSpan))
		},
	)
	return clientConfig
}

func createKubernetesClient(logger logr.Logger, rateLimitQPS float64, rateLimitBurst int, opts ...kubeclient.NewOption) kubernetes.Interface {
	logger = logger.WithName("kube-client")
	logger.Info("create kube client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := kubeclient.NewForConfig(createClientConfig(logger, rateLimitQPS, rateLimitBurst), opts...)
	checkError(logger, err, "failed to create kubernetes client")
	return client
}

func createKyvernoClient(logger logr.Logger, opts ...kyverno.NewOption) versioned.Interface {
	logger = logger.WithName("kyverno-client")
	logger.Info("create kyverno client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := kyverno.NewForConfig(createClientConfig(logger, clientRateLimitQPS, clientRateLimitBurst), opts...)
	checkError(logger, err, "failed to create kyverno client")
	return client
}

func createDynamicClient(logger logr.Logger, opts ...dyn.NewOption) dynamic.Interface {
	logger = logger.WithName("dynamic-client")
	logger.Info("create dynamic client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := dyn.NewForConfig(createClientConfig(logger, clientRateLimitQPS, clientRateLimitBurst), opts...)
	checkError(logger, err, "failed to create dynamic client")
	return client
}

func createMetadataClient(logger logr.Logger, opts ...meta.NewOption) metadata.Interface {
	logger = logger.WithName("metadata-client")
	logger.Info("create metadata client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := meta.NewForConfig(createClientConfig(logger, clientRateLimitQPS, clientRateLimitBurst), opts...)
	checkError(logger, err, "failed to create metadata client")
	return client
}

func createApiServerClient(logger logr.Logger, opts ...apisrv.NewOption) apiserver.Interface {
	logger = logger.WithName("apiserver-client")
	logger.Info("create apiserver client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := apisrv.NewForConfig(createClientConfig(logger, clientRateLimitQPS, clientRateLimitBurst), opts...)
	checkError(logger, err, "failed to create apiserver client")
	return client
}

func createKyvernoDynamicClient(logger logr.Logger, ctx context.Context, dyn dynamic.Interface, kube kubernetes.Interface, resync time.Duration) dclient.Interface {
	logger = logger.WithName("d-client")
	logger.Info("create the kyverno dynamic client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := dclient.NewClient(ctx, dyn, kube, resync)
	checkError(logger, err, "failed to create d client")
	return client
}

func createEventsClient(logger logr.Logger, metricsManager metrics.MetricsConfigManager) eventsv1.EventsV1Interface {
	logger = logger.WithName("events-client")
	logger.Info("create the events client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client := kubeclient.From(createKubernetesClient(logger, eventsRateLimitQPS, eventsRateLimitBurst), kubeclient.WithTracing())
	client = client.WithMetrics(metricsManager, metrics.KubeClient)
	return client.EventsV1()
}

func CreateAggregatorClient(logger logr.Logger, opts ...agg.NewOption) aggregator.Interface {
	logger = logger.WithName("aggregator-client")
	logger.Info("create aggregator client...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	client, err := agg.NewForConfig(createClientConfig(logger, clientRateLimitQPS, clientRateLimitBurst), opts...)
	checkError(logger, err, "failed to create aggregator client")
	return client
}
