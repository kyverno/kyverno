package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	controller "github.com/nirmata/kyverno/pkg/controller"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	gencontroller "github.com/nirmata/kyverno/pkg/gencontroller"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/nirmata/kyverno/pkg/violation"
	"github.com/nirmata/kyverno/pkg/webhooks"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig    string
	serverIP      string
	filterK8Kinds webhooks.ArrayFlags
)

func main() {
	defer glog.Flush()

	printVersionInfo()
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v\n", err)
	}

	client, err := client.NewClient(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating client: %v\n", err)
	}

	policyInformerFactory, err := sharedinformer.NewSharedInformerFactory(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating policy sharedinformer: %v\n", err)
	}
	kubeInformer := utils.NewKubeInformerFactory(clientConfig)
	eventController := event.NewEventController(client, policyInformerFactory)
	violationBuilder := violation.NewPolicyViolationBuilder(client, policyInformerFactory, eventController)

	policyController := controller.NewPolicyController(
		client,
		policyInformerFactory,
		violationBuilder,
		eventController)

	genControler := gencontroller.NewGenController(client, eventController, policyInformerFactory, violationBuilder, kubeInformer.Core().V1().Namespaces())
	tlsPair, err := initTLSPemPair(clientConfig, client)
	if err != nil {
		glog.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}
	server, err := webhooks.NewWebhookServer(client, tlsPair, policyInformerFactory, eventController, violationBuilder, filterK8Kinds)
	if err != nil {
		glog.Fatalf("Unable to create webhook server: %v\n", err)
	}

	webhookRegistrationClient, err := webhooks.NewWebhookRegistrationClient(clientConfig, client, serverIP)
	if err != nil {
		glog.Fatalf("Unable to register admission webhooks on cluster: %v\n", err)
	}

	stopCh := signals.SetupSignalHandler()

	policyInformerFactory.Run(stopCh)
	kubeInformer.Start(stopCh)
	eventController.Run(stopCh)
	genControler.Run(stopCh)
	if err = policyController.Run(stopCh); err != nil {
		glog.Fatalf("Error running PolicyController: %v\n", err)
	}

	if err = webhookRegistrationClient.Register(); err != nil {
		glog.Fatalf("Failed registering Admission Webhooks: %v\n", err)
	}

	server.RunAsync()
	<-stopCh
	server.Stop()
	genControler.Stop()
	eventController.Stop()
	policyController.Stop()
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.Var(&filterK8Kinds, "filterKind", "k8 kind where policy is not evaluated by the admission webhook. example --filterKind \"Event\" --filterKind \"TokenReview,ClusterRole\"")
	config.LogDefaultFlags()
	flag.Parse()
}
