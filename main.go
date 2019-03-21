package main

import (
	"flag"
	"log"

	"github.com/nirmata/kube-policy/controller"
	"github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/webhooks"
	"github.com/nirmata/kube-policy/server"

	signals "k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig string
	cert       string
	key        string
)

func main() {
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v\n", err)
	}

	err = webhooks.RegisterMutationWebhook(clientConfig)
	if err != nil {
		log.Fatalf("Error registering mutation webhook server: %v\n", err)
	}

	controller, err := controller.NewPolicyController(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating PolicyController: %s\n", err)
	}

	kubeclient, err := kubeclient.NewKubeClient(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating kubeclient: %v\n", err)
	}

	tlsPair := readTlsPairFromFiles(cert, key)
	if tlsPair != nil {
		log.Print("Using given TLS key/certificate pair")
	} else {
		tlsPair, err = initTlsPemsPair(clientConfig, kubeclient)
		if err != nil {
			log.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
		}
	}

	serverConfig := server.WebhookServerConfig{
		TlsPemPair: tlsPair,
		Controller: controller,
		Kubeclient: kubeclient,
	}
	server, err := server.NewWebhookServer(serverConfig, nil)
	if err != nil {
		log.Fatalf("Unable to create webhook server: %v\n", err)
	}
	server.RunAsync()

	stopCh := signals.SetupSignalHandler()
	controller.Run(stopCh)

	if err != nil {
		log.Fatalf("Error running PolicyController! Error: %s\n", err)
		return
	}

	log.Println("Policy Controller has started")
	<-stopCh
	server.Stop()
	log.Println("Policy Controller has stopped")
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
	flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
	flag.Parse()
}
