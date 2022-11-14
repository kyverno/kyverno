package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
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
)

const (
	resyncPeriod = 15 * time.Minute
)

func parseFlags() error {
	logging.Init(nil)
	flag.StringVar(&logFormat, "loggingFormat", logging.TextFormat, "This determines the output format of the logger.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 20, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 50, "Configure the maximum burst for throttle. Uses the client default if zero.")
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
	_, kubeClient, err := createKubeClients(logger)
	if err != nil {
		os.Exit(1)
	}
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))

	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
	server := NewServer(
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
