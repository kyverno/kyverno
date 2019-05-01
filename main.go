package main

import (
	"flag"

	"github.com/nirmata/kube-policy/controller"
	"github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/server"
	"github.com/nirmata/kube-policy/webhooks"

	"k8s.io/klog"
	signals "k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig string
	cert       string
	key        string
)

func main() {
	initializeLogger()
	defer klog.Flush()

	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v\n", err)
	}
	controller, err := controller.NewPolicyController(clientConfig)
	if err != nil {
		klog.Fatalf("Error creating PolicyController: %s\n", err)
	}

	kubeclient, err := kubeclient.NewKubeClient(clientConfig)
	if err != nil {
		klog.Fatalf("Error creating kubeclient: %v\n", err)
	}

	mutationWebhook, err := webhooks.CreateMutationWebhook(clientConfig, kubeclient, controller)
	if err != nil {
		klog.Fatalf("Error creating mutation webhook: %v\n", err)
	}

	tlsPair, err := initTLSPemPair(cert, key, clientConfig, kubeclient)
	if err != nil {
		klog.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}

	server, err := server.NewWebhookServer(tlsPair, mutationWebhook)
	if err != nil {
		klog.Fatalf("Unable to create webhook server: %v\n", err)
	}
	server.RunAsync()

	stopCh := signals.SetupSignalHandler()
	controller.Run(stopCh)

	if err != nil {
		klog.Fatalf("Error running PolicyController: %s\n", err)
	}
	klog.Info("Policy controller started")

	<-stopCh

	server.Stop()
	klog.Info("Policy controller stopped")
}

// Initialize logger
func initializeLogger() {
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
	flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
	flag.Parse()
}
