package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/profile"

	"github.com/golang/glog"
	client "github.com/nirmata/kyverno/pkg/dclient"
	tls "github.com/nirmata/kyverno/pkg/tls"
	"github.com/nirmata/kyverno/pkg/version"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

func printVersionInfo() {
	v := version.GetVersion()
	glog.Infof("Kyverno version: %s\n", v.BuildVersion)
	glog.Infof("Kyverno BuildHash: %s\n", v.BuildHash)
	glog.Infof("Kyverno BuildTime: %s\n", v.BuildTime)
}

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		glog.Info("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	glog.Infof("Using configuration from '%s'", kubeconfig)
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func initTLSPemPair(configuration *rest.Config, client *client.Client) (*tls.TlsPemPair, error) {
	certProps, err := client.GetTLSCertProps(configuration)
	if err != nil {
		return nil, err
	}
	tlsPair := client.ReadTlsPair(certProps)
	if tls.IsTlsPairShouldBeUpdated(tlsPair) {
		glog.Info("Generating new key/certificate pair for TLS")
		tlsPair, err = client.GenerateTlsPemPair(certProps)
		if err != nil {
			return nil, err
		}
		if err = client.WriteTlsPair(certProps, tlsPair); err != nil {
			return nil, fmt.Errorf("Unable to save TLS pair to the cluster: %v", err)
		}
		return tlsPair, nil
	}

	glog.Infoln("Using existing TLS key/certificate pair")
	return tlsPair, nil
}

var prof interface {
	Stop()
}

func enableProfiling(cpu, memory bool) interface {
	Stop()
} {

	file := "/opt/nirmata/kyverno/" + randomString(6)
	if cpu {
		glog.Infof("Enable cpu profiling ...")
		prof = profile.Start(profile.CPUProfile, profile.ProfilePath(file))
	} else if memory {
		glog.Infof("Enable memory profiling ...")
		prof = profile.Start(profile.MemProfile, profile.ProfilePath(file))
	}

	return prof
}

func disableProfiling(p interface{ Stop() }) {
	if p != nil {
		p.Stop()
	}
}

// generate random string
const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func randomString(length int) string {
	return stringWithCharset(length, charset)
}
