/*
Cleans up stale webhookconfigurations created by kyverno that were not cleanedup
*/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"sync"
	"time"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/signal"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	setupLog             = log.Log.WithName("setup")
	clientRateLimitQPS   float64
	clientRateLimitBurst int

	updateLabelSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      policyreport.LabelSelectorKey,
				Operator: metav1.LabelSelectorOpDoesNotExist,
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
	convertGenerateRequest         string = "ConvertGenerateRequest"
)

func main() {
	// clear flags initialized in static dependencies
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	klog.InitFlags(nil) // add the block above before invoking klog.InitFlags()
	log.SetLogger(klogr.New())
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 0, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 0, "Configure the maximum burst for throttle. Uses the client default if zero.")
	if err := flag.Set("v", "2"); err != nil {
		klog.Fatalf("failed to set log level: %v", err)
	}

	flag.Parse()

	// os signal handler
	stopCh := signal.SetupSignalHandler()
	// create client config
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		setupLog.Error(err, "Failed to create clientConfig")
		os.Exit(1)
	}
	if err := config.ConfigureClientConfig(clientConfig, clientRateLimitQPS, clientRateLimitBurst); err != nil {
		setupLog.Error(err, "Failed to create clientConfig")
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := dclient.NewClient(clientConfig, kubeClient, 15*time.Minute, stopCh)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	pclient, err := kyvernoclient.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// Exit for unsupported version of kubernetes cluster
	if !utils.HigherThanKubernetesVersion(kubeClient.Discovery(), log.Log, 1, 16, 0) {
		os.Exit(1)
	}

	requests := []request{
		{policyReportKind},
		{clusterPolicyReportKind},

		{reportChangeRequestKind},
		{clusterReportChangeRequestKind},

		{convertGenerateRequest},
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
		name := tls.GenerateRootCASecretName()
		_, err = kubeClient.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			log.Log.Info("failed to fetch root CA secret", "name", name, "error", err.Error())
			if !errors.IsNotFound(err) {
				os.Exit(1)
			}
		}

		name = tls.GenerateTLSPairSecretName()
		_, err = kubeClient.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			log.Log.Info("failed to fetch TLS Pair secret", "name", name, "error", err.Error())
			if !errors.IsNotFound(err) {
				os.Exit(1)
			}
		}

		if err = acquireLeader(ctx, kubeClient); err != nil {
			log.Log.Info("Failed to create lease 'kyvernopre-lock'")
			os.Exit(1)
		}

		// use pipline to pass request to cleanup resources
		in := gen(done, stopCh, requests...)
		// process requests
		// processing routine count : 2
		p1 := process(client, pclient, done, stopCh, in)
		p2 := process(client, pclient, done, stopCh, in)
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

		os.Exit(0)
	}

	le, err := leaderelection.New("kyvernopre", config.KyvernoNamespace(), kubeClient, run, nil, log.Log.WithName("kyvernopre/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	le.Run(ctx)
}

func acquireLeader(ctx context.Context, kubeClient kubernetes.Interface) error {
	_, err := kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()).Get(ctx, "kyvernopre-lock", metav1.GetOptions{})
	if err != nil {
		log.Log.Info("Lease 'kyvernopre-lock' not found. Starting clean-up...")
	} else {
		log.Log.Info("Leader was elected, quitting")
		os.Exit(0)
	}

	lease := coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyvernopre-lock",
		},
	}
	_, err = kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()).Create(ctx, &lease, metav1.CreateOptions{})

	return err
}

func executeRequest(client dclient.Interface, kyvernoclient kyvernoclient.Interface, req request) error {
	switch req.kind {
	case policyReportKind:
		return removePolicyReport(client, req.kind)
	case clusterPolicyReportKind:
		return removeClusterPolicyReport(client, req.kind)
	case reportChangeRequestKind:
		return removeReportChangeRequest(client, req.kind)
	case clusterReportChangeRequestKind:
		return removeClusterReportChangeRequest(client, req.kind)
	case convertGenerateRequest:
		return convertGR(kyvernoclient)
	}

	return nil
}

type request struct {
	kind string
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
func process(client dclient.Interface, kyvernoclient kyvernoclient.Interface, done <-chan struct{}, stopCh <-chan struct{}, requests <-chan request) <-chan error {
	logger := log.Log.WithName("process")
	out := make(chan error)
	go func() {
		defer close(out)
		for req := range requests {
			select {
			case out <- executeRequest(client, kyvernoclient, req):
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

func removeClusterPolicyReport(client dclient.Interface, kind string) error {
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

func removePolicyReport(client dclient.Interface, kind string) error {
	logger := log.Log.WithName("removePolicyReport")

	polrs, err := client.ListResource("", kind, metav1.NamespaceAll, policyreport.LabelSelector)
	if err != nil {
		logger.Error(err, "failed to list policyReport")
		return nil
	}

	for _, polr := range polrs.Items {
		deleteResource(client, polr.GetAPIVersion(), polr.GetKind(), polr.GetNamespace(), polr.GetName())
	}

	return nil
}

// Deprecated: New ClusterPolicyReports already has required labels, will be removed in
// 1.8.0 version
func addClusterPolicyReportSelectorLabel(client dclient.Interface) {
	logger := log.Log.WithName("addClusterPolicyReportSelectorLabel")

	cpolrs, err := client.ListResource("", clusterPolicyReportKind, "", updateLabelSelector)
	if err != nil {
		logger.Error(err, "failed to list clusterPolicyReport")
		return
	}

	for _, cpolr := range cpolrs.Items {
		if cpolr.GetName() == policyreport.GeneratePolicyReportName("", "") {
			addSelectorLabel(client, cpolr.GetAPIVersion(), cpolr.GetKind(), "", cpolr.GetName())
		}
	}
}

// Deprecated: New PolicyReports already has required labels, will be removed in
// 1.8.0 version
func addPolicyReportSelectorLabel(client dclient.Interface) {
	logger := log.Log.WithName("addPolicyReportSelectorLabel")

	polrs, err := client.ListResource("", policyReportKind, metav1.NamespaceAll, updateLabelSelector)
	if err != nil {
		logger.Error(err, "failed to list policyReport")
		return
	}

	for _, polr := range polrs.Items {
		if polr.GetName() == policyreport.GeneratePolicyReportName(polr.GetNamespace(), "") {
			addSelectorLabel(client, polr.GetAPIVersion(), polr.GetKind(), polr.GetNamespace(), polr.GetName())
		}
	}
}

func removeReportChangeRequest(client dclient.Interface, kind string) error {
	logger := log.Log.WithName("removeReportChangeRequest")

	ns := config.KyvernoNamespace()
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

func removeClusterReportChangeRequest(client dclient.Interface, kind string) error {
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

func deleteResource(client dclient.Interface, apiversion, kind, ns, name string) {
	err := client.DeleteResource(apiversion, kind, ns, name, false)
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to delete resource", "kind", kind, "name", name)
		return
	}

	log.Log.Info("successfully cleaned up resource", "kind", kind, "name", name)
}

func addSelectorLabel(client dclient.Interface, apiversion, kind, ns, name string) {
	res, err := client.GetResource(apiversion, kind, ns, name)
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to get resource", "kind", kind, "name", name)
		return
	}

	l, err := metav1.LabelSelectorAsMap(policyreport.LabelSelector)
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

func convertGR(pclient kyvernoclient.Interface) error {
	logger := log.Log.WithName("convertGenerateRequest")

	var errors []error
	grs, err := pclient.KyvernoV1().GenerateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "failed to list update requests")
		return err
	}
	for _, gr := range grs.Items {
		cp := gr.DeepCopy()
		var request *admissionv1.AdmissionRequest
		if cp.Spec.Context.AdmissionRequestInfo.AdmissionRequest != "" {
			var r admissionv1.AdmissionRequest
			err := json.Unmarshal([]byte(cp.Spec.Context.AdmissionRequestInfo.AdmissionRequest), &r)
			if err != nil {
				logger.Error(err, "failed to unmarshal admission request")
				errors = append(errors, err)
				continue
			}
		}
		ur := &kyvernov1beta1.UpdateRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ur-",
				Namespace:    config.KyvernoNamespace(),
				Labels:       cp.GetLabels(),
			},
			Spec: kyvernov1beta1.UpdateRequestSpec{
				Type:     kyvernov1beta1.Generate,
				Policy:   cp.Spec.Policy,
				Resource: cp.Spec.Resource,
				Context: kyvernov1beta1.UpdateRequestSpecContext{
					UserRequestInfo: kyvernov1beta1.RequestInfo{
						Roles:             cp.Spec.Context.UserRequestInfo.Roles,
						ClusterRoles:      cp.Spec.Context.UserRequestInfo.ClusterRoles,
						AdmissionUserInfo: cp.Spec.Context.UserRequestInfo.AdmissionUserInfo,
					},
					AdmissionRequestInfo: kyvernov1beta1.AdmissionRequestInfoObject{
						AdmissionRequest: request,
						Operation:        cp.Spec.Context.AdmissionRequestInfo.Operation,
					},
				},
			},
		}

		_, err := pclient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
		if err != nil {
			logger.Info("failed to create UpdateRequest", "GR namespace", gr.GetNamespace(), "GR name", gr.GetName(), "err", err.Error())
			errors = append(errors, err)
			continue
		} else {
			logger.Info("successfully created UpdateRequest", "GR namespace", gr.GetNamespace(), "GR name", gr.GetName())
		}

		if err := pclient.KyvernoV1().GenerateRequests(config.KyvernoNamespace()).Delete(context.TODO(), gr.GetName(), metav1.DeleteOptions{}); err != nil {
			errors = append(errors, err)
			logger.Error(err, "failed to delete GR")
		}
	}

	err = multierr.Combine(errors...)
	return err
}
