package main

import (
	"io/ioutil"
	"log"
	"net/url"

	"github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/constants"
	"github.com/nirmata/kube-policy/utils"

	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
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

func readTlsPairFromFiles(certFile, keyFile string) *utils.TlsPemPair {
	if certFile == "" || keyFile == "" {
		return nil
	}

	certContent, err := ioutil.ReadFile(certFile)
	if err != nil {
		log.Printf("Unable to read file with TLS certificate: path - %s, error - %v", certFile, err)
		return nil
	}

	keyContent, err := ioutil.ReadFile(keyFile)
	if err != nil {
		log.Printf("Unable to read file with TLS private key: path - %s, error - %v", keyFile, err)
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
	apiServerUrl, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}
	certProps := utils.TlsCertificateProps{
		Service:       constants.WebhookServiceName,
		Namespace:     constants.WebhookServiceNamespace,
		ApiServerHost: apiServerUrl.Hostname(),
	}

	tlsPair := client.ReadTlsPair(certProps)
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
