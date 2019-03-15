package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/url"

	"github.com/nirmata/kube-policy/controller"
	"github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/server"
	"github.com/nirmata/kube-policy/utils"

	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	signals "k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig string
	cert       string
	key        string
)

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		log.Printf("Using in-cluster configuration")
		return rest.InClusterConfig()
	} else {
		log.Printf("Using configuration from '%s'", kubeconfig)
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
}

func readTlsPairFromFiles() *utils.TlsPemPair {
	certContent, err := ioutil.ReadFile(cert)
	if err != nil {
		log.Printf("Unable to read file with TLS certificate: %v", err)
		return nil
	}

	keyContent, err := ioutil.ReadFile(key)
	if err != nil {
		log.Printf("Unable to read file with TLS private key: %v", err)
		return nil
	}

	return &utils.TlsPemPair{
		Certificate: certContent,
		PrivateKey:  keyContent,
	}
}

// Loads or creates PEM private key and TLS certificate for webhook server
// Returns struct with key/certificate pair
func initTlsPemsPair(config *rest.Config, client *kubeclient.KubeClient) (*utils.TlsPemPair, error) {
	tlsPair := readTlsPairFromFiles()
	if tlsPair != nil {
		log.Print("Using given TLS key/certificate pair")
		return tlsPair, nil
	}

	apiServerUrl, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}
	certProps := utils.TlsCertificateProps{
		Service:       "localhost",
		Namespace:     "default",
		ApiServerHost: apiServerUrl.Hostname(),
	}

	tlsPair = client.ReadTlsPair(certProps)
	if utils.IsTlsPairShouldBeUpdated(tlsPair) {
		log.Printf("Generating new key/certificate pair for TLS")
		tlsPair, err = client.GenerateTlsPemPair(certProps)
		if err != nil {
			return nil, err
		}
		err = client.WriteTlsPair(certProps, tlsPair)
		if err != nil {
			log.Printf("Unable to save TLS pair to the cluster: %v", err)
		}
	}

	return tlsPair, nil
}

func main() {
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v\n", err)
	}

	controller, err := controller.NewPolicyController(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating PolicyController! Error: %s\n", err)
	}

	kubeclient, err := kubeclient.NewKubeClient(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating kubeclient: %v\n", err)
	}

	tlsPair, err := initTlsPemsPair(clientConfig, kubeclient)
	if err != nil {
		log.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
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
