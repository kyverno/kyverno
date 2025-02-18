package leaderelection

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const DefaultRetryPeriod = 2 * time.Second

type Interface interface {
	// Run is a blocking call that runs a leader election
	Run(ctx context.Context)

	// ID returns this instances unique identifier
	ID() string

	// Name returns the name of the leader election
	Name() string

	// Namespace is the Kubernetes namespace used to coordinate the leader election
	Namespace() string

	// IsLeader indicates if this instance is the leader
	IsLeader() bool

	// GetLeader returns the leader ID
	GetLeader() string
}

type config struct {
	name              string
	namespace         string
	startWork         func(context.Context)
	stopWork          func()
	kubeClient        kubernetes.Interface
	lock              resourcelock.Interface
	leaderElectionCfg leaderelection.LeaderElectionConfig
	leaderElector     *leaderelection.LeaderElector
	isLeader          int64
	log               logr.Logger
}

func New(log logr.Logger, name, namespace string, kubeClient kubernetes.Interface, id string, retryPeriod time.Duration, startWork func(context.Context), stopWork func()) (Interface, error) {
	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		namespace,
		name,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing resource lock: %s/%s: %w", namespace, name, err)
	}
	e := &config{
		name:       name,
		namespace:  namespace,
		kubeClient: kubeClient,
		lock:       lock,
		startWork:  startWork,
		stopWork:   stopWork,
		log:        log.WithValues("id", lock.Identity()),
	}
	e.leaderElectionCfg = leaderelection.LeaderElectionConfig{
		Lock:            e.lock,
		ReleaseOnCancel: false,
		LeaseDuration:   6 * retryPeriod,
		RenewDeadline:   5 * retryPeriod,
		RetryPeriod:     retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				atomic.StoreInt64(&e.isLeader, 1)
				e.log.V(2).Info("started leading")
				if e.startWork != nil {
					e.startWork(ctx)
				}
			},
			OnStoppedLeading: func() {
				atomic.StoreInt64(&e.isLeader, 0)
				e.log.V(2).Info("leadership lost, stopped leading")
				if e.stopWork != nil {
					e.stopWork()
				}
			},
			OnNewLeader: func(identity string) {
				if identity == e.lock.Identity() {
					e.log.V(4).Info("still leading")
				} else {
					e.log.V(2).Info("another instance has been elected as leader", "leader", identity)
				}
			},
		},
	}
	e.leaderElector, err = leaderelection.NewLeaderElector(e.leaderElectionCfg)
	if err != nil {
		e.log.Error(err, "failed to create leaderElector")
		os.Exit(1)
	}
	if e.leaderElectionCfg.WatchDog != nil {
		e.leaderElectionCfg.WatchDog.SetLeaderElection(e.leaderElector)
	}
	return e, nil
}

func (e *config) Name() string {
	return e.name
}

func (e *config) Namespace() string {
	return e.namespace
}

func (e *config) ID() string {
	return e.lock.Identity()
}

func (e *config) IsLeader() bool {
	return atomic.LoadInt64(&e.isLeader) == 1
}

func (e *config) GetLeader() string {
	return e.leaderElector.GetLeader()
}

func (e *config) Run(ctx context.Context) {
	e.leaderElector.Run(ctx)
}
