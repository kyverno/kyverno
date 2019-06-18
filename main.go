package main

import (
	"flag"
	"strings"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	controller "github.com/nirmata/kyverno/pkg/controller"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	"github.com/nirmata/kyverno/pkg/violation"
	"github.com/nirmata/kyverno/pkg/webhooks"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig    string
	serverIP      string
	filterK8Kinds arrayFlags
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
	eventController := event.NewEventController(client, policyInformerFactory)
	violationBuilder := violation.NewPolicyViolationBuilder(client, policyInformerFactory, eventController)

	policyController := controller.NewPolicyController(
		client,
		policyInformerFactory,
		violationBuilder,
		eventController)

	tlsPair, err := initTLSPemPair(clientConfig, client)
	if err != nil {
		glog.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}

	server, err := webhooks.NewWebhookServer(client, tlsPair, policyInformerFactory, filterK8Kinds)
	if err != nil {
		glog.Fatalf("Unable to create webhook server: %v\n", err)
	}

	webhookRegistrationClient, err := webhooks.NewWebhookRegistrationClient(clientConfig, client, serverIP)
	if err != nil {
		glog.Fatalf("Unable to register admission webhooks on cluster: %v\n", err)
	}

	stopCh := signals.SetupSignalHandler()

	policyInformerFactory.Run(stopCh)
	eventController.Run(stopCh)

	if err = policyController.Run(stopCh); err != nil {
		glog.Fatalf("Error running PolicyController: %v\n", err)
	}

	if err = webhookRegistrationClient.Register(); err != nil {
		glog.Fatalf("Failed registering Admission Webhooks: %v\n", err)
	}

	server.RunAsync()
	<-stopCh
	server.Stop()
	policyController.Stop()
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	var sb strings.Builder
	for _, str := range *i {
		sb.WriteString(str)
	}
	return sb.String()
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}
func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.Var(&filterK8Kinds, "filterKind", "k8 kinds where polcies are not to be applied on")
	config.LogDefaultFlags()
	flag.Parse()
}
