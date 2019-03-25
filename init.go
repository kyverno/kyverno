package main

import (
	"io/ioutil"
	"log"
	"net/url"

	"github.com/nirmata/kube-policy/config"
	"github.com/nirmata/kube-policy/kubeclient"
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

func initTlsPemPair(certFile, keyFile string, clientConfig *rest.Config, kubeclient *kubeclient.KubeClient) (*utils.TlsPemPair, error) {
	var tlsPair *utils.TlsPemPair
	if certFile != "" || keyFile != "" {
		tlsPair = tlsPairFromFiles(certFile, keyFile)
	}

	var err error
	if tlsPair != nil {
		log.Print("Using given TLS key/certificate pair")
		return tlsPair, nil
	} else {
		tlsPair, err = tlsPairFromCluster(clientConfig, kubeclient)
		if err == nil {
			log.Printf("Using TLS key/certificate from cluster")
		}
		return tlsPair, err
	}
}

// Loads PEM private key and TLS certificate from given files
func tlsPairFromFiles(certFile, keyFile string) *utils.TlsPemPair {
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

// Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func tlsPairFromCluster(configuration *rest.Config, client *kubeclient.KubeClient) (*utils.TlsPemPair, error) {
	apiServerUrl, err := url.Parse(configuration.Host)
	if err != nil {
		return nil, err
	}
	certProps := utils.TlsCertificateProps{
		Service:       config.WebhookServiceName,
		Namespace:     config.KubePolicyNamespace,
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
