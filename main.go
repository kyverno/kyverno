package main

import (
	"flag"
	"log"

	"k8s.io/sample-controller/pkg/signals"

	client "github.com/nirmata/kube-policy/client"
	policyclientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions"
	controller "github.com/nirmata/kube-policy/pkg/controller"
	event "github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/violation"
	"github.com/nirmata/kube-policy/pkg/webhooks"
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

	client, err := client.NewDynamicClient(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating client: %v\n", err)
	}

	policyClientset, err := policyclientset.NewForConfig(clientConfig)
	if err != nil {
		log.Fatalf("Error creating policyClient: %v\n", err)
	}

	//TODO wrap the policyInformer inside a factory
	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, 0)
	policyInformer := policyInformerFactory.Kubepolicy().V1alpha1().Policies()

	eventController := event.NewEventController(client, policyInformer.Lister(), nil)
	violationBuilder := violation.NewPolicyViolationBuilder(client, policyInformer.Lister(), policyClientset, eventController, nil)

	policyController := controller.NewPolicyController(
		client,
		policyInformer,
		violationBuilder,
		eventController,
		nil)

	tlsPair, err := initTlsPemPair(cert, key, clientConfig, client)
	if err != nil {
		log.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}

	server, err := webhooks.NewWebhookServer(tlsPair, policyInformer.Lister(), nil)
	if err != nil {
		log.Fatalf("Unable to create webhook server: %v\n", err)
	}

	webhookRegistrationClient, err := webhooks.NewWebhookRegistrationClient(clientConfig, client)
	if err != nil {
		log.Fatalf("Unable to register admission webhooks on cluster: %v\n", err)
	}

	stopCh := signals.SetupSignalHandler()

	policyInformerFactory.Start(stopCh)
	eventController.Run(stopCh)

	if err = policyController.Run(stopCh); err != nil {
		log.Fatalf("Error running PolicyController: %v\n", err)
	}

	if err = webhookRegistrationClient.Register(); err != nil {
		log.Fatalf("Failed registering Admission Webhooks: %v\n", err)
	}

	server.RunAsync()
	<-stopCh
	server.Stop()
	policyController.Stop()
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
	flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
	flag.Parse()
}
