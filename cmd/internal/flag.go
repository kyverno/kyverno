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

func initKubeconfigFlags() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 20, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 50, "Configure the maximum burst for throttle. Uses the client default if zero.")
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
	// kubeconfig
	if config.UsesKubeconfig() {
		initKubeconfigFlags()
	}
}
