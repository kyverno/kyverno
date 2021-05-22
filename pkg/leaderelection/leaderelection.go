package leaderelection

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

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
}

type Config struct {
	name       string
	namespace  string
	id         string
	startWork  func()
	stopWork   func()
	kubeClient kubernetes.Interface
	lock       resourcelock.Interface
	isLeader   int64
	log        logr.Logger
}

func New(name, namespace string, kubeClient kubernetes.Interface, startWork, stopWork func(), log logr.Logger) (Interface, error) {
	id, err := os.Hostname()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting host name", "namespace", namespace, "name", name)
	}

	id = id + "_" + string(uuid.NewUUID())

	lock, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		namespace,
		name,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
	)

	if err != nil {
		return nil, errors.Wrapf(err, "error initializing resource lock", "namespace", namespace, "name", name)
	}

	return &Config{
		name:       name,
		namespace:  namespace,
		kubeClient: kubeClient,
		lock:       lock,
		startWork:  startWork,
		stopWork:   stopWork,
		log:        log,
	}, nil
}

func (e *Config) Name() string {
	return e.name
}

func (e *Config) Namespace() string {
	return e.namespace
}

func (e *Config) IsLeader() bool {
	return atomic.LoadInt64(&e.isLeader) == 1
}

func (e *Config) ID() string {
	return e.lock.Identity()
}

func (e *Config) Run(ctx context.Context) {

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            e.lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				atomic.StoreInt64(&e.isLeader, 1)
				e.log.WithValues("id", e.lock.Identity()).Info("started leading")
				if e.startWork != nil {
					e.startWork()
				}
			},

			OnStoppedLeading: func() {
				atomic.StoreInt64(&e.isLeader, 0)
				e.log.WithValues("id", e.lock.Identity()).Info("stopped leading")
				if e.stopWork != nil {
					e.stopWork()
				}
			},

			OnNewLeader: func(identity string) {
				if identity == e.lock.Identity() {
					return
				}

				e.log.WithValues("current id", e.lock.Identity(), "leader", identity).Info("another instance has been elected as leader")
			},
		},
	})
}
