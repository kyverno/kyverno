/*
Cleans up stale webhookconfigurations created by kyverno that were not cleanedup
*/
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/signal"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	kubeconfig string
	setupLog   = log.Log.WithName("setup")
)

const (
	policyReportKind               string = "PolicyReport"
	clusterPolicyReportKind        string = "ClusterPolicyReport"
	reportChangeRequestKind        string = "ReportChangeRequest"
	clusterReportChangeRequestKind string = "ClusterReportChangeRequest"
	policyViolation                string = "PolicyViolation"
	clusterPolicyViolation         string = "ClusterPolicyViolation"
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
	client, err := client.NewClient(clientConfig, 15*time.Minute, stopCh, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// Exit for unsupported version of kubernetes cluster
	if !utils.HigherThanKubernetesVersion(client, log.Log, 1, 16, 0) {
		os.Exit(1)
	}

	requests := []request{
		{policyReportKind, ""},
		{clusterPolicyReportKind, ""},

		{reportChangeRequestKind, ""},
		{clusterReportChangeRequestKind, ""},

		// clean up policy violation CRD
		{policyViolation, ""},
		{clusterPolicyViolation, ""},
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
		log.Log.Info("failed to cleanup prior configurations")
		os.Exit(1)
	}
}

func executeRequest(client *client.Client, req request) error {
	switch req.kind {
	case policyReportKind:
		return removePolicyReport(client, req.kind)
	case clusterPolicyReportKind:
		return removeClusterPolicyReport(client, req.kind)
	case reportChangeRequestKind:
		return removeReportChangeRequest(client, req.kind)
	case clusterReportChangeRequestKind:
		return removeClusterReportChangeRequest(client, req.kind)
	case policyViolation, clusterPolicyViolation:
		return removeViolationCRD(client)
	}

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
			case out <- executeRequest(client, req):
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

func removeClusterPolicyReport(client *client.Client, kind string) error {
	logger := log.Log.WithName("removeClusterPolicyReport")

	cpolrs, err := client.ListResource("", kind, "", nil)
	if err != nil {
		logger.Error(err, "failed to list clusterPolicyReport")
		return nil
	}

	for _, cpolr := range cpolrs.Items {
		deleteResource(client, cpolr.GetAPIVersion(), cpolr.GetKind(), "", cpolr.GetName(), nil)
	}
	return nil
}

func removePolicyReport(client *client.Client, kind string) error {
	logger := log.Log.WithName("removePolicyReport")

	namespaces, err := client.ListResource("", "Namespace", "", nil)
	if err != nil {
		logger.Error(err, "failed to list namespaces")
		return err
	}

	// name of namespace policy report follows the name convention
	// pr-ns-<namespace name>
	for _, ns := range namespaces.Items {
		reportNames := []string{
			fmt.Sprintf("policyreport-ns-%s", ns.GetName()),
			fmt.Sprintf("pr-ns-%s", ns.GetName()),
			fmt.Sprintf("polr-ns-%s", ns.GetName()),
		}

		var wg sync.WaitGroup
		wg.Add(len(reportNames))
		for _, reportName := range reportNames {
			go deleteResource(client, "", kind, ns.GetName(), reportName, &wg)
		}
		wg.Wait()
	}

	return nil
}

func removeReportChangeRequest(client *client.Client, kind string) error {
	logger := log.Log.WithName("removeReportChangeRequest")

	ns := getKyvernoNameSpace()
	rcrList, err := client.ListResource("", kind, ns, nil)
	if err != nil {
		logger.Error(err, "failed to list reportChangeRequest")
		return nil
	}

	for _, rcr := range rcrList.Items {
		deleteResource(client, rcr.GetAPIVersion(), rcr.GetKind(), rcr.GetNamespace(), rcr.GetName(), nil)
	}

	return nil
}

func removeClusterReportChangeRequest(client *client.Client, kind string) error {
	crcrList, err := client.ListResource("", kind, "", nil)
	if err != nil {
		log.Log.Error(err, "failed to list clusterReportChangeRequest")
		return nil
	}

	for _, crcr := range crcrList.Items {
		deleteResource(client, crcr.GetAPIVersion(), crcr.GetKind(), "", crcr.GetName(), nil)
	}
	return nil
}

func removeViolationCRD(client *client.Client) error {
	if err := client.DeleteResource("", "CustomResourceDefinition", "", "policyviolations.kyverno.io", false); err != nil {
		if !errors.IsNotFound(err) {
			log.Log.Error(err, "failed to delete CRD policyViolation")
		}
	}

	if err := client.DeleteResource("", "CustomResourceDefinition", "", "clusterpolicyviolations.kyverno.io", false); err != nil {
		if !errors.IsNotFound(err) {
			log.Log.Error(err, "failed to delete CRD clusterPolicyViolation")
		}
	}
	return nil
}

// getKubePolicyNameSpace - setting default KubePolicyNameSpace
func getKyvernoNameSpace() string {
	kyvernoNamespace := os.Getenv("KYVERNO_NAMESPACE")
	if kyvernoNamespace == "" {
		kyvernoNamespace = "kyverno"
	}
	return kyvernoNamespace
}

func deleteResource(client *client.Client, apiversion, kind, ns, name string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	err := client.DeleteResource(apiversion, kind, ns, name, false)
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to delete resource", "kind", kind, "name", name)
		return
	}

	log.Log.Info("successfully cleaned up resource", "kind", kind, "name", name)
}
