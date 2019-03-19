package main

import (
	"io/ioutil"
	"log"
	"net/url"

	"github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/utils"

	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
const serviceName string = "kube-policy-svc"
const namespace string = "default"

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
	certContent, err := ioutil.ReadFile(certFile)
	if err != nil {
		log.Printf("Unable to read file with TLS certificate: %v", err)
		return nil
	}

	keyContent, err := ioutil.ReadFile(keyFile)
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
	apiServerUrl, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}
	certProps := utils.TlsCertificateProps{
		Service:       serviceName,
		Namespace:     namespace,
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