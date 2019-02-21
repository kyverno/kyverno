package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nirmata/kube-policy/controller"
	"github.com/nirmata/kube-policy/server"

	"k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
	cert       string
	key        string
)

func main() {
	flag.Parse()

	if cert == "" || key == "" {
		log.Fatal("TLS certificate or/and key is not set")
	}

	crdcLogger := log.New(os.Stdout, "Policy Controller: ", log.LstdFlags|log.Lshortfile)
	controller, err := controller.NewPolicyController(masterURL, kubeconfig, crdcLogger)
	if err != nil {
		fmt.Printf("Error creating PolicyController! Error: %s\n", err)
		return
	}

	httpLogger := log.New(os.Stdout, "HTTPS Server: ", log.LstdFlags|log.Lshortfile)
	server := server.NewWebhookServer(cert, key, controller, httpLogger)
	server.RunAsync()

	stopCh := signals.SetupSignalHandler()
	controller.Run(stopCh)

	if err != nil {
		fmt.Printf("Error running PolicyController! Error: %s\n", err)
	}

	fmt.Printf("Policy PolicyController has started")
	<-stopCh
	server.Stop()
	fmt.Printf("Policy PolicyController has stopped")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
	flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
}
