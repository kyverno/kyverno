/*
Cleans up stale webhookconfigurations created by kyverno that were not cleanedup
*/
package main

import (
	"flag"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/signal"
	"k8s.io/apimachinery/pkg/api/errors"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig string
)

const (
	mutatingWebhookConfigKind   string = "MutatingWebhookConfiguration"
	validatingWebhookConfigKind string = "ValidatingWebhookConfiguration"
)

func main() {
	defer glog.Flush()
	// os signal handler
	stopCh := signal.SetupSignalHandler()
	// create client config
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v\n", err)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := client.NewClient(clientConfig, 10*time.Second, stopCh)
	if err != nil {
		glog.Fatalf("Error creating client: %v\n", err)
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
			glog.Errorf("failed to cleanup: %v", err)
		}
	}
	// if there is any failure then we fail process
	if failure {
		glog.Errorf("failed to cleanup webhook configurations")
		os.Exit(1)
	}
}

func init() {
	// arguments
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.Set("logtostderr", "true")
	flag.Set("stderrthreshold", "WARNING")
	flag.Set("v", "2")
	flag.Parse()
}

func removeWebhookIfExists(client *client.Client, kind string, name string) error {
	var err error
	// Get resource
	_, err = client.GetResource(kind, "", name)
	if errors.IsNotFound(err) {
		glog.V(4).Infof("%s(%s) not found", name, kind)
		return nil
	}
	if err != nil {
		glog.Errorf("failed to get resource %s(%s)", name, kind)
		return err
	}
	// Delete resource
	err = client.DeleteResource(kind, "", name, false)
	if err != nil {
		glog.Errorf("failed to delete resource %s(%s)", name, kind)
		return err
	}
	glog.Infof("cleaned up resource %s(%s)", name, kind)
	return nil
}

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		glog.Info("Using in-cluster configuration")
		return rest.InClusterConfig()
	}
	glog.Infof("Using configuration from '%s'", kubeconfig)
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
	out := make(chan error)
	go func() {
		defer close(out)
		for req := range requests {
			select {
			case out <- removeWebhookIfExists(client, req.kind, req.name):
			case <-done:
				println("done process")
				return
			case <-stopCh:
				println("shutting down process")
				return
			}
		}
	}()
	return out
}

// waits for all processes to be complete and merges result
func merge(done <-chan struct{}, stopCh <-chan struct{}, processes ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)
	// gets the output from each process
	output := func(ch <-chan error) {
		defer wg.Done()
		for err := range ch {
			select {
			case out <- err:
			case <-done:
				println("done merge")
				return
			case <-stopCh:
				println("shutting down merge")
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
	serverVersion, err := client.DiscoveryClient.GetServerVersion()
	if err != nil {
		glog.Fatalf("Failed to get kubernetes server version: %v\n", err)
	}
	exp := regexp.MustCompile(`v(\d*).(\d*).(\d*)`)
	groups := exp.FindAllStringSubmatch(serverVersion.String(), -1)
	if len(groups) != 1 || len(groups[0]) != 4 {
		glog.Fatalf("Failed to extract kubernetes server version: %v\n", serverVersion, err)
	}
	// convert string to int
	// assuming the version are always intergers
	major, err := strconv.Atoi(groups[0][1])
	if err != nil {
		glog.Fatalf("Failed to extract kubernetes major server version: %v\n", serverVersion, err)
	}
	minor, err := strconv.Atoi(groups[0][2])
	if err != nil {
		glog.Fatalf("Failed to extract kubernetes minor server version: %v\n", serverVersion, err)
	}
	sub, err := strconv.Atoi(groups[0][3])
	if err != nil {
		glog.Fatalf("Failed to extract kubernetes sub minor server version: %v\n", serverVersion, err)
	}
	if major <= 1 && minor <= 12 && sub < 7 {
		glog.Fatalf("Unsupported kubernetes server version %s. Kyverno is supported from version v1.12.7+", serverVersion)
	}
}
