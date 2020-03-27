/*
Cleans up stale webhookconfigurations created by kyverno that were not cleanedup
*/
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/signal"
	"k8s.io/apimachinery/pkg/api/errors"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	kubeconfig string
	setupLog   = log.Log.WithName("setup")
)

const (
	mutatingWebhookConfigKind   string = "MutatingWebhookConfiguration"
	validatingWebhookConfigKind string = "ValidatingWebhookConfiguration"
)

func main() {
	klog.InitFlags(nil)
	log.SetLogger(klogr.New())
	// arguments
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	if err := flag.Set("v", "2"); err != nil {
		klog.Fatalf("failed to set log level: %v", err)
	}
	flag.Parse()

	// os signal handler
	stopCh := signal.SetupSignalHandler()
	// create client config
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		setupLog.Error(err, "Failed to build kubeconfig")
		os.Exit(1)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := client.NewClient(clientConfig, 10*time.Second, stopCh, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// Exit for unsupported version of kubernetes cluster
	// https://github.com/nirmata/kyverno/issues/700
	// - supported from v1.12.7+
	isVersionSupported(client)

	requests := []request{
		// Resource
		{validatingWebhookConfigKind, config.ValidatingWebhookConfigurationName},
		{validatingWebhookConfigKind, config.ValidatingWebhookConfigurationDebugName},
		{mutatingWebhookConfigKind, config.MutatingWebhookConfigurationName},
		{mutatingWebhookConfigKind, config.MutatingWebhookConfigurationDebugName},
		// Policy
		{validatingWebhookConfigKind, config.PolicyValidatingWebhookConfigurationName},
		{validatingWebhookConfigKind, config.PolicyValidatingWebhookConfigurationDebugName},
		{mutatingWebhookConfigKind, config.PolicyMutatingWebhookConfigurationName},
		{mutatingWebhookConfigKind, config.PolicyMutatingWebhookConfigurationDebugName},
	}

	done := make(chan struct{})
	defer close(done)
	failure := false
	// use pipline to pass request to cleanup resources
	// generate requests
	in := gen(done, stopCh, requests...)
	// process requests
	// processing routine count : 2
	p1 := process(client, done, stopCh, in)
	p2 := process(client, done, stopCh, in)
	// merge results from processing routines
	for err := range merge(done, stopCh, p1, p2) {
		if err != nil {
			failure = true
			log.Log.Error(err, "failed to cleanup resource")
		}
	}
	// if there is any failure then we fail process
	if failure {
		log.Log.Info("failed to cleanup webhook configurations")
		os.Exit(1)
	}
}

<<<<<<< HEAD
=======
func init() {
	// arguments
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	err := flag.Set("logtostderr", "true")
	if err != nil {
		glog.Errorf("failed to set flag %v", err)
	}
	err = flag.Set("stderrthreshold", "WARNING")
	if err != nil {
		glog.Errorf("failed to set flag %v", err)
	}
	err = flag.Set("v", "2")
	if err != nil {
		glog.Errorf("failed to set flag %v", err)
	}
	flag.Parse()
}
>>>>>>> 010bc2b43d99e27daf8709baca5b02ac5ca10011

func removeWebhookIfExists(client *client.Client, kind string, name string) error {
	logger := log.Log.WithName("removeExistingWebhook").WithValues("kind", kind, "name", name)
	var err error
	// Get resource
	_, err = client.GetResource(kind, "", name)
	if errors.IsNotFound(err) {
		logger.V(4).Info("resource not found")
		return nil
	}
	if err != nil {
		logger.Error(err, "failed to get resource")
		return err
	}
	// Delete resource
	err = client.DeleteResource(kind, "", name, false)
	if err != nil {
		logger.Error(err, "failed to delete resource")
		return err
	}
	logger.Info("removed the resource")
	return nil
}

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	logger := log.Log
	if kubeconfig == "" {
		logger.Info("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	logger.Info(fmt.Sprintf("Using configuration from '%s'", kubeconfig))
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

type request struct {
	kind string
	name string
}

/* Processing Pipeline
  					-> Process Requests
Generate Requests	-> Process Requests		-> Merge Results
					-> Process Requests
- number of processes can be controlled
- stop processing on SIGTERM OR SIGNKILL signal
- stop processing if any process fails(supported)
*/
// Generates requests to be processed
func gen(done <-chan struct{}, stopCh <-chan struct{}, requests ...request) <-chan request {
	out := make(chan request)
	go func() {
		defer close(out)
		for _, req := range requests {
			select {
			case out <- req:
			case <-done:
				println("done generate")
				return
			case <-stopCh:
				println("shutting down generate")
				return
			}
		}
	}()
	return out
}

// processes the requests
func process(client *client.Client, done <-chan struct{}, stopCh <-chan struct{}, requests <-chan request) <-chan error {
	logger := log.Log.WithName("process")
	out := make(chan error)
	go func() {
		defer close(out)
		for req := range requests {
			select {
			case out <- removeWebhookIfExists(client, req.kind, req.name):
			case <-done:
				logger.Info("done")
				return
			case <-stopCh:
				logger.Info("shutting down")
				return
			}
		}
	}()
	return out
}

// waits for all processes to be complete and merges result
func merge(done <-chan struct{}, stopCh <-chan struct{}, processes ...<-chan error) <-chan error {
	logger := log.Log.WithName("merge")
	var wg sync.WaitGroup
	out := make(chan error)
	// gets the output from each process
	output := func(ch <-chan error) {
		defer wg.Done()
		for err := range ch {
			select {
			case out <- err:
			case <-done:
				logger.Info("done")
				return
			case <-stopCh:
				logger.Info("shutting down")
				return
			}
		}
	}

	wg.Add(len(processes))
	for _, process := range processes {
		go output(process)
	}

	// close when all the process goroutines are done
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func isVersionSupported(client *client.Client) {
	logger := log.Log
	serverVersion, err := client.DiscoveryClient.GetServerVersion()
	if err != nil {
		logger.Error(err, "Failed to get kubernetes server version")
		os.Exit(1)
	}
	exp := regexp.MustCompile(`v(\d*).(\d*).(\d*)`)
	groups := exp.FindAllStringSubmatch(serverVersion.String(), -1)
	if len(groups) != 1 || len(groups[0]) != 4 {
		logger.Error(err, "Failed to extract kubernetes server version", "serverVersion", serverVersion)
		os.Exit(1)
	}
	// convert string to int
	// assuming the version are always intergers
	major, err := strconv.Atoi(groups[0][1])
	if err != nil {
		logger.Error(err, "Failed to extract kubernetes major server version", "serverVersion", serverVersion)
		os.Exit(1)
	}
	minor, err := strconv.Atoi(groups[0][2])
	if err != nil {
		logger.Error(err, "Failed to extract kubernetes minor server version", "serverVersion", serverVersion)
		os.Exit(1)
	}
	sub, err := strconv.Atoi(groups[0][3])
	if err != nil {
		logger.Error(err, "Failed to extract kubernetes sub minor server version", "serverVersion", serverVersion)
		os.Exit(1)
	}
	if major <= 1 && minor <= 12 && sub < 7 {
		logger.Info("Unsupported kubernetes server version %s. Kyverno is supported from version v1.12.7+", "serverVersion", serverVersion)
		os.Exit(1)
	}
}
