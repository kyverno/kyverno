package main

import (
	"io/ioutil"
	"log"
	"net/url"

	client "github.com/nirmata/kube-policy/client"
	"github.com/nirmata/kube-policy/pkg/config"
	tls "github.com/nirmata/kube-policy/pkg/tls"

	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		log.Printf("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	log.Printf("Using configuration from '%s'", kubeconfig)
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func initTlsPemPair(certFile, keyFile string, clientConfig *rest.Config, client *client.Client) (*tls.TlsPemPair, error) {
	var tlsPair *tls.TlsPemPair
	if certFile != "" || keyFile != "" {
		tlsPair = tlsPairFromFiles(certFile, keyFile)
	}

	var err error
	if tlsPair != nil {
		return tlsPair, nil
	}
	tlsPair, err = tlsPairFromCluster(clientConfig, client)
	return tlsPair, err
}

// Loads PEM private key and TLS certificate from given files
func tlsPairFromFiles(certFile, keyFile string) *tls.TlsPemPair {
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

	return &tls.TlsPemPair{
		Certificate: certContent,
		PrivateKey:  keyContent,
	}
}

// Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func tlsPairFromCluster(configuration *rest.Config, client *client.Client) (*tls.TlsPemPair, error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return nil, err
	}
	certProps := tls.TlsCertificateProps{
		Service:       config.WebhookServiceName,
		Namespace:     config.KubePolicyNamespace,
		ApiServerHost: apiServerURL.Hostname(),
	}
	tlsPair := client.ReadTlsPair(certProps)
	if tls.IsTlsPairShouldBeUpdated(tlsPair) {
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
