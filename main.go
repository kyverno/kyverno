package main

import (
    "log"
    "os"
    "flag"
    "fmt"

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

    httpLogger := log.New(os.Stdout, "http: ", log.LstdFlags|log.Lshortfile)
    crdcLogger := log.New(os.Stdout, "crdc: ", log.LstdFlags|log.Lshortfile)

	server := server.NewWebhookServer(cert, key, httpLogger)
	server.RunAsync()

    controller, err := controller.NewController(masterURL, kubeconfig, crdcLogger)
    if err != nil {
        fmt.Printf("Error creating Controller! Error: %s\n", err)
        return
    }

    stopCh := signals.SetupSignalHandler()
    controller.Run(stopCh)

    if err != nil {
        fmt.Printf("Error running Controller! Error: %s\n", err)
    }

    fmt.Printf("Policy Controller has started")
    <-stopCh
    server.Stop()
	fmt.Printf("Policy Controller has stopped")
}

func init() {
    flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
    flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
    flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
    flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
}