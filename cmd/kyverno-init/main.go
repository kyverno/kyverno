/*
Cleans up stale webhookconfigurations created by kyverno that were not cleanedup
*/
package main

import (
	"context"
	"os"
	"sync"

	"github.com/kyverno/kyverno/cmd/internal"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	policyReportKind        string = "PolicyReport"
	clusterPolicyReportKind string = "ClusterPolicyReport"
)

func main() {
	// config
	appConfig := internal.NewConfiguration(
		internal.WithKubeconfig(),
		internal.WithKyvernoClient(),
		internal.WithDynamicClient(),
		internal.WithKyvernoDynamicClient(),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup logger
	// show version
	// start profiling
	// setup signals
	// setup maxprocs
	ctx, setup, sdown := internal.Setup(appConfig, "kyverno-init-controller", false)
	defer sdown()
	// Exit for unsupported version of kubernetes cluster
	if !kubeutils.HigherThanKubernetesVersion(setup.KubeClient.Discovery(), logging.GlobalLogger(), 1, 16, 0) {
		os.Exit(1)
	}
	requests := []request{
		{policyReportKind},
		{clusterPolicyReportKind},
	}

	go func() {
		defer sdown()
		<-ctx.Done()
	}()

	done := make(chan struct{})
	defer close(done)
	failure := false

	run := func(context.Context) {
		if err := acquireLeader(ctx, setup.KubeClient); err != nil {
			logging.V(2).Info("Failed to create lease 'kyvernopre-lock'")
			os.Exit(1)
		}

		// use pipeline to pass request to cleanup resources
		in := gen(done, ctx.Done(), requests...)
		// process requests
		// processing routine count : 2
		p1 := process(setup.KyvernoDynamicClient, setup.KyvernoClient, done, ctx.Done(), in)
		p2 := process(setup.KyvernoDynamicClient, setup.KyvernoClient, done, ctx.Done(), in)
		// merge results from processing routines
		for err := range merge(done, ctx.Done(), p1, p2) {
			if err != nil {
				failure = true
				logging.Error(err, "failed to cleanup resource")
			}
		}
		// if there is any failure then we fail process
		if failure {
			logging.V(2).Info("failed to cleanup prior configurations")
			os.Exit(1)
		}

		os.Exit(0)
	}

	le, err := leaderelection.New(
		logging.WithName("kyvernopre/LeaderElection"),
		"kyvernopre",
		config.KyvernoNamespace(),
		setup.KubeClient,
		config.KyvernoPodName(),
		leaderelection.DefaultRetryPeriod,
		run,
		nil,
	)
	if err != nil {
		setup.Logger.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	le.Run(ctx)
}

func acquireLeader(ctx context.Context, kubeClient kubernetes.Interface) error {
	_, err := kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()).Get(ctx, "kyvernopre-lock", metav1.GetOptions{})
	if err != nil {
		logging.V(2).Info("Lease 'kyvernopre-lock' not found. Starting clean-up...")
	} else {
		logging.V(2).Info("Leader was elected, quitting")
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
	logger := logging.WithName("process")
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
	logger := logging.WithName("merge")
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
