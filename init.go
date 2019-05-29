package main

import (
	"log"

	client "github.com/nirmata/kyverno/pkg/dclient"
	tls "github.com/nirmata/kyverno/pkg/tls"
	"github.com/nirmata/kyverno/pkg/version"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

func printVersionInfo() {
	v := version.GetVersion()
	log.Printf("Kyverno version: %s\n", v.BuildVersion)
	log.Printf("Kyverno BuildHash: %s\n", v.BuildHash)
	log.Printf("Kyverno BuildTime: %s\n", v.BuildTime)
}

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		log.Printf("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	log.Printf("Using configuration from '%s'", kubeconfig)
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func initTlsPemPair(configuration *rest.Config, client *client.Client) (*tls.TlsPemPair, error) {
	certProps, err := client.GetTLSCertProps(configuration)
	if err != nil {
		return nil, err
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
