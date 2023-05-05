package internal

import (
	"flag"

	"github.com/kyverno/kyverno/pkg/logging"
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

func initKubeconfigFlags() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 300, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 300, "Configure the maximum burst for throttle. Uses the client default if zero.")
}

func InitFlags(config Configuration) {
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
		initKubeconfigFlags()
	}
	for _, flagset := range config.FlagSets() {
		flagset.VisitAll(func(f *flag.Flag) {
			flag.CommandLine.Var(f.Value, f.Name, f.Usage)
		})
	}
}

func ParseFlags(config Configuration) {
	InitFlags(config)
	flag.Parse()
}
