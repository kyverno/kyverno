/*
Cleans up stale webhookconfigurations created by kyverno that were not cleanedup
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/signal"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/utils"
	coord "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	kubeconfig           string
	setupLog             = log.Log.WithName("setup")
	clientRateLimitQPS   float64
	clientRateLimitBurst int

	updateLabelSelector = &v1.LabelSelector{
		MatchExpressions: []v1.LabelSelectorRequirement{
			{
				Key:      policyreport.LabelSelectorKey,
				Operator: v1.LabelSelectorOpDoesNotExist,
				Values:   []string{},
			},
		},
	}
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
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 0, "Configure the maximum QPS to the master from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 0, "Configure the maximum burst for throttle. Uses the client default if zero.")
	if err := flag.Set("v", "2"); err != nil {
		klog.Fatalf("failed to set log level: %v", err)
	}

	flag.Parse()

	// os signal handler
	stopCh := signal.SetupSignalHandler()
	// create client config
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst, log.Log)
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

	kubeClientLeaderElection, err := utils.NewKubeClient(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-stopCh
		cancel()
	}()

	addPolicyReportSelectorLabel(client)
	addClusterPolicyReportSelectorLabel(client)

	done := make(chan struct{})
	defer close(done)
	failure := false

	run := func() {
		certProps, err := tls.GetTLSCertProps(clientConfig)
		if err != nil {
			log.Log.Info("failed to get cert properties: %v", err.Error())
			os.Exit(1)
		}

		depl, err := client.GetResource("", "Deployment", getKyvernoNameSpace(), config.KyvernoDeploymentName)
		deplHash := ""
		if err != nil {
			log.Log.Info("failed to fetch deployment '%v': %v", config.KyvernoDeploymentName, err.Error())
			os.Exit(1)
		}
		deplHash = fmt.Sprintf("%v", depl.GetUID())

		name := tls.GenerateRootCASecretName(certProps)
		secretUnstr, err := client.GetResource("", "Secret", getKyvernoNameSpace(), name)
		if err != nil {
			log.Log.Info("failed to fetch secret '%v': %v", name, err.Error())

			if !errors.IsNotFound(err) {
				os.Exit(1)
			}
		} else if tls.CanAddAnnotationToSecret(deplHash, secretUnstr) {
			secretUnstr.SetAnnotations(map[string]string{tls.MasterDeploymentUID: deplHash})
			_, err = client.UpdateResource("", "Secret", certProps.Namespace, secretUnstr, false)
			if err != nil {
				log.Log.Info("failed to update cert: %v", err.Error())
				os.Exit(1)
			}
		}

		name = tls.GenerateTLSPairSecretName(certProps)
		secretUnstr, err = client.GetResource("", "Secret", getKyvernoNameSpace(), name)
		if err != nil {
			log.Log.Info("failed to fetch secret '%v': %v", name, err.Error())

			if !errors.IsNotFound(err) {
				os.Exit(1)
			}
		} else if tls.CanAddAnnotationToSecret(deplHash, secretUnstr) {
			secretUnstr.SetAnnotations(map[string]string{tls.MasterDeploymentUID: deplHash})
			_, err = client.UpdateResource("", "Secret", certProps.Namespace, secretUnstr, false)
			if err != nil {
				log.Log.Info("failed to update cert: %v", err.Error())
				os.Exit(1)
			}
		}

		_, err = kubeClientLeaderElection.CoordinationV1().Leases(getKyvernoNameSpace()).Get(ctx, "kyvernopre-lock", v1.GetOptions{})
		if err != nil {
			log.Log.Info("Lease 'kyvernopre-lock' not found. Starting clean-up...")
		} else {
			log.Log.Info("Clean-up complete. Leader exiting...")
			os.Exit(0)
		}

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

		lease := coord.Lease{}
		lease.ObjectMeta.Name = "kyvernopre-lock"
		_, err = kubeClientLeaderElection.CoordinationV1().Leases(getKyvernoNameSpace()).Create(ctx, &lease, v1.CreateOptions{})
		if err != nil {
			log.Log.Info("Failed to create lease 'kyvernopre-lock'")
		}

		log.Log.Info("Clean-up complete. Leader exiting...")

		os.Exit(0)
	}

	le, err := leaderelection.New("kyvernopre", getKyvernoNameSpace(), kubeClientLeaderElection, run, nil, log.Log.WithName("kyvernopre/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	le.Run(ctx)
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

	cpolrs, err := client.ListResource("", kind, "", policyreport.LabelSelector)
	if err != nil {
		logger.Error(err, "failed to list clusterPolicyReport")
		return nil
	}

	for _, cpolr := range cpolrs.Items {
		deleteResource(client, cpolr.GetAPIVersion(), cpolr.GetKind(), "", cpolr.GetName())
	}
	return nil
}

func removePolicyReport(client *client.Client, kind string) error {
	logger := log.Log.WithName("removePolicyReport")

	polrs, err := client.ListResource("", kind, v1.NamespaceAll, policyreport.LabelSelector)
	if err != nil {
		logger.Error(err, "failed to list policyReport")
		return nil
	}

	for _, polr := range polrs.Items {
		deleteResource(client, polr.GetAPIVersion(), polr.GetKind(), polr.GetNamespace(), polr.GetName())
	}

	return nil
}

func addClusterPolicyReportSelectorLabel(client *client.Client) {
	logger := log.Log.WithName("addClusterPolicyReportSelectorLabel")

	cpolrs, err := client.ListResource("", clusterPolicyReportKind, "", updateLabelSelector)
	if err != nil {
		logger.Error(err, "failed to list clusterPolicyReport")
		return
	}

	for _, cpolr := range cpolrs.Items {
		if cpolr.GetName() == policyreport.GeneratePolicyReportName("") {
			addSelectorLabel(client, cpolr.GetAPIVersion(), cpolr.GetKind(), "", cpolr.GetName())
		}
	}
}

func addPolicyReportSelectorLabel(client *client.Client) {
	logger := log.Log.WithName("addPolicyReportSelectorLabel")

	polrs, err := client.ListResource("", policyReportKind, v1.NamespaceAll, updateLabelSelector)
	if err != nil {
		logger.Error(err, "failed to list policyReport")
		return
	}

	for _, polr := range polrs.Items {
		if polr.GetName() == policyreport.GeneratePolicyReportName(polr.GetNamespace()) {
			addSelectorLabel(client, polr.GetAPIVersion(), polr.GetKind(), polr.GetNamespace(), polr.GetName())
		}
	}
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
		deleteResource(client, rcr.GetAPIVersion(), rcr.GetKind(), rcr.GetNamespace(), rcr.GetName())
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
		deleteResource(client, crcr.GetAPIVersion(), crcr.GetKind(), "", crcr.GetName())
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

func deleteResource(client *client.Client, apiversion, kind, ns, name string) {
	err := client.DeleteResource(apiversion, kind, ns, name, false)
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to delete resource", "kind", kind, "name", name)
		return
	}

	log.Log.Info("successfully cleaned up resource", "kind", kind, "name", name)
}

func addSelectorLabel(client *client.Client, apiversion, kind, ns, name string) {
	res, err := client.GetResource(apiversion, kind, ns, name)
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to get resource", "kind", kind, "name", name)
		return
	}

	l, err := v1.LabelSelectorAsMap(policyreport.LabelSelector)
	if err != nil {
		log.Log.Error(err, "failed to convert labels", "labels", policyreport.LabelSelector)
		return
	}

	res.SetLabels(labels.Merge(res.GetLabels(), l))

	_, err = client.UpdateResource(apiversion, kind, ns, res, false)
	if err != nil {
		log.Log.Error(err, "failed to update resource", "kind", kind, "name", name)
		return
	}

	log.Log.Info("successfully updated resource labels", "kind", kind, "name", name)
}
