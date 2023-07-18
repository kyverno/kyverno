package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"sync"
	"time"

	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	vapcontroller "github.com/kyverno/kyverno/pkg/controllers/vapgeneration"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const (
	resyncPeriod = 15 * time.Minute
)

func createrLeaderControllers(
	vapWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
) ([]internal.Controller, error) {
	vapController := vapcontroller.NewController(
		kubeClient,
		kyvernoClient,
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kubeInformer.Admissionregistration().V1alpha1().ValidatingAdmissionPolicies(),
		kubeInformer.Admissionregistration().V1alpha1().ValidatingAdmissionPolicyBindings(),
	)
	return []internal.Controller{
		internal.NewController(vapcontroller.ControllerName, vapController, vapWorkers),
	}, nil
}

func main() {
	var (
		vapWorkers      int
		maxQueuedEvents int
	)
	flagset := flag.NewFlagSet("vap-controller", flag.ExitOnError)
	flagset.IntVar(&vapWorkers, "vapWorkers", 3, "Workers for the vap controller.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithConfigMapCaching(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup
	ctx, setup, sdown := internal.Setup(appConfig, "kyverno-vap-controller", false)
	defer sdown()

	// informer factories
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
	eventGenerator := event.NewEventGenerator(
		setup.KyvernoDynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		[]string{},
		logging.WithName("EventGenerator"),
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kyvernoInformer) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// start event generator
	var wg sync.WaitGroup
	go eventGenerator.Run(ctx, 3, &wg)
	// setup leader election
	le, err := leaderelection.New(
		setup.Logger.WithName("leader-election"),
		"kyverno-vap-controller",
		config.KyvernoNamespace(),
		setup.LeaderElectionClient,
		config.KyvernoPodName(),
		internal.LeaderElectionRetryPeriod(),
		func(ctx context.Context) {
			logger := setup.Logger.WithName("leader")
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, resyncPeriod)
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
			// create leader controllers
			leaderControllers, err := createrLeaderControllers(
				vapWorkers,
				kubeInformer,
				kyvernoInformer,
				setup.KubeClient,
				setup.KyvernoClient,
			)

			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(ctx, logger, kyvernoInformer, kubeInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			// start leader controllers
			var wg sync.WaitGroup
			for _, controller := range leaderControllers {
				controller.Run(ctx, logger.WithName("controllers"), &wg)
			}
			// wait all controllers shut down
			wg.Wait()
		},
		nil,
	)
	if err != nil {
		setup.Logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// start leader election
	le.Run(ctx)
	wg.Wait()
}
