package internal

import (
	"flag"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/sigstore/sigstore/pkg/tuf"
)

var (
	// logging
	loggingFormat string
	// profiling
	profilingEnabled bool
	profilingAddress string
	profilingPort    string
	// tracing
	tracingEnabled bool
	tracingAddress string
	tracingPort    string
	tracingCreds   string
	// metrics
	otel                 string
	otelCollector        string
	metricsPort          string
	transportCreds       string
	disableMetricsExport bool
	// kubeconfig
	kubeconfig           string
	clientRateLimitQPS   float64
	clientRateLimitBurst int
	// engine
	enablePolicyException  bool
	exceptionNamespace     string
	enableConfigMapCaching bool
	// cosign
	imageSignatureRepository string
	enableTUF                bool
	tufMirror                string
	tufRoot                  string
	// registry client
	imagePullSecrets          string
	allowInsecureRegistry     bool
	registryCredentialHelpers string
	// leader election
	leaderElectionRetryPeriod time.Duration
	// image verify cache
	imageVerifyCacheEnabled     bool
	imageVerifyCacheTTLDuration time.Duration
	imageVerifyCacheMaxSize     int64
)

func initLoggingFlags() {
	logging.InitFlags(nil)
	flag.StringVar(&loggingFormat, "loggingFormat", logging.TextFormat, "This determines the output format of the logger.")
	checkErr(flag.Set("v", "2"), "failed to init flags")
}

func initProfilingFlags() {
	flag.BoolVar(&profilingEnabled, "profile", false, "Set this flag to 'true', to enable profiling.")
	flag.StringVar(&profilingPort, "profilePort", "6060", "Profiling server port, defaults to '6060'.")
	flag.StringVar(&profilingAddress, "profileAddress", "", "Profiling server address, defaults to ''.")
}

func initTracingFlags() {
	flag.BoolVar(&tracingEnabled, "enableTracing", false, "Set this flag to 'true', to enable tracing.")
	flag.StringVar(&tracingPort, "tracingPort", "4317", "Tracing receiver port, defaults to '4317'.")
	flag.StringVar(&tracingAddress, "tracingAddress", "", "Tracing receiver address, defaults to ''.")
	flag.StringVar(&tracingCreds, "tracingCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Tracing Client. If empty string is set, means an insecure connection will be used")
}

func initMetricsFlags() {
	flag.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flag.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flag.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
}

func initKubeconfigFlags(qps float64, burst int) {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", qps, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", burst, "Configure the maximum burst for throttle. Uses the client default if zero.")
}

func initPolicyExceptionsFlags() {
	flag.StringVar(&exceptionNamespace, "exceptionNamespace", "", "Configure the namespace to accept PolicyExceptions.")
	flag.BoolVar(&enablePolicyException, "enablePolicyException", true, "Enable PolicyException feature.")
}

func initConfigMapCachingFlags() {
	flag.BoolVar(&enableConfigMapCaching, "enableConfigMapCaching", true, "Enable config maps caching.")
}

func initDeferredLoadingFlags() {
	flag.Func(toggle.EnableDeferredLoadingFlagName, toggle.EnableDeferredLoadingDescription, toggle.EnableDeferredLoading.Parse)
}

func initCosignFlags() {
	flag.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "(DEPRECATED, will be removed in 1.12) Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flag.BoolVar(&enableTUF, "enableTuf", false, "enable tuf for private sigstore deployments")
	flag.StringVar(&tufMirror, "tufMirror", tuf.DefaultRemoteRoot, "Alternate TUF mirror for sigstore. If left blank, public sigstore one is used for cosign verification.")
	flag.StringVar(&tufRoot, "tufRoot", "", "Alternate TUF root.json for sigstore. If left blank, public sigstore one is used for cosign verification.")
}

func initRegistryClientFlags() {
	flag.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flag.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flag.StringVar(&registryCredentialHelpers, "registryCredentialHelpers", "", "Credential helpers to enable (default,google,amazon,azure,github). No helpers are added when this flag is empty.")
}

func initImageVerifyCacheFlags() {
	flag.BoolVar(&imageVerifyCacheEnabled, "imageVerifyCacheEnabled", true, "Whether to use a TTL cache for storing verified images.")
	flag.Int64Var(&imageVerifyCacheMaxSize, "imageVerifyCacheMaxSize", 1000, "Max size limit for the TTL cache, 0 means default 1000 size limit.")
	flag.DurationVar(&imageVerifyCacheTTLDuration, "imageVerifyCacheTTLDuration", 60*time.Minute, "Max TTL value for a cache, 0 means default 1 hour TTL.")
}

func initLeaderElectionFlags() {
	flag.DurationVar(&leaderElectionRetryPeriod, "leaderElectionRetryPeriod", leaderelection.DefaultRetryPeriod, "Configure leader election retry period.")
}

type options struct {
	clientRateLimitQPS   float64
	clientRateLimitBurst int
}

func newOptions() options {
	return options{
		clientRateLimitQPS:   20,
		clientRateLimitBurst: 50,
	}
}

type Option = func(*options)

func WithDefaultQps(qps float64) Option {
	return func(o *options) {
		o.clientRateLimitQPS = qps
	}
}

func WithDefaultBurst(burst int) Option {
	return func(o *options) {
		o.clientRateLimitBurst = burst
	}
}

func initFlags(config Configuration, opts ...Option) {
	options := newOptions()
	for _, o := range opts {
		if o != nil {
			o(&options)
		}
	}
	// logging
	initLoggingFlags()
	// profiling
	if config.UsesProfiling() {
		initProfilingFlags()
	}
	// tracing
	if config.UsesTracing() {
		initTracingFlags()
	}
	// metrics
	if config.UsesMetrics() {
		initMetricsFlags()
	}
	// kubeconfig
	if config.UsesKubeconfig() {
		initKubeconfigFlags(options.clientRateLimitQPS, options.clientRateLimitBurst)
	}
	// policy exceptions
	if config.UsesPolicyExceptions() {
		initPolicyExceptionsFlags()
	}
	// config map caching
	if config.UsesConfigMapCaching() {
		initConfigMapCachingFlags()
	}
	// deferred loading
	if config.UsesDeferredLoading() {
		initDeferredLoadingFlags()
	}
	// cosign
	if config.UsesCosign() {
		initCosignFlags()
	}
	// registry client
	if config.UsesRegistryClient() {
		initRegistryClientFlags()
	}
	// image verify cache
	if config.UsesImageVerifyCache() {
		initImageVerifyCacheFlags()
	}
	// leader election
	if config.UsesLeaderElection() {
		initLeaderElectionFlags()
	}
	for _, flagset := range config.FlagSets() {
		flagset.VisitAll(func(f *flag.Flag) {
			flag.CommandLine.Var(f.Value, f.Name, f.Usage)
		})
	}
}

func showWarnings(config Configuration, logger logr.Logger) {
	if config.UsesCosign() {
		if imageSignatureRepository != "" {
			logger.Info("Warning: imageSignatureRepository is deprecated and will be removed in 1.12. Use per rule configuration `verifyImages.Repository` instead.")
		}
	}
}

func ParseFlags(config Configuration, opts ...Option) {
	initFlags(config, opts...)
	flag.Parse()
}

func ExceptionNamespace() string {
	return exceptionNamespace
}

func PolicyExceptionEnabled() bool {
	return enablePolicyException
}

func LeaderElectionRetryPeriod() time.Duration {
	return leaderElectionRetryPeriod
}

func printFlagSettings(logger logr.Logger) {
	logger = logger.WithName("flag")
	flag.VisitAll(func(f *flag.Flag) {
		logger.V(2).Info("", f.Name, f.Value)
	})
}
