package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/nirmata/kube-policy/controller"
	"github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/server"

	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	signals "k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
	cert       string
	key        string
)

func createClientConfig(masterURL, kubeconfig string) (*rest.Config, error) {
	// TODO: make possible to create config within a cluster with proper rights
	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func main() {
	flag.Parse()

	if cert == "" || key == "" {
		log.Fatal("TLS certificate or/and key is not set")
	}

	clientConfig, err := createClientConfig(masterURL, kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v\n", err)
		return
	}

	controller, err := controller.NewPolicyController(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating PolicyController! Error: %s\n", err)
		return
	}

	kubeclient, err := kubeclient.NewKubeClient(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating kubeclient: %v\n", err)
	}

	serverConfig := server.WebhookServerConfig{
		CertFile:   cert,
		KeyFile:    key,
		Controller: controller,
		Kubeclient: kubeclient,
	}

	server, err := server.NewWebhookServer(serverConfig, nil)
	server.RunAsync()

	stopCh := signals.SetupSignalHandler()
	controller.Run(stopCh)

	if err != nil {
		log.Fatalf("Error running PolicyController! Error: %s\n", err)
		return
	}

	fmt.Println("Policy Controller has started")
	<-stopCh
	server.Stop()
	fmt.Println("Policy Controller has stopped")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
	flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
}
