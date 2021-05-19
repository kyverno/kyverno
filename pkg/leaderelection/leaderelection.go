package leaderelection

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type Config struct {
	name       string
	namespace  string
	startWork  func()
	stopWork   func()
	kubeClient kubernetes.Interface
	log        logr.Logger
	isLeader   int64
}

func (e *Config) IsLeader() bool {
	return atomic.LoadInt64(&e.isLeader) == 1
}

func (e *Config) Run(ctx context.Context) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id, err := os.Hostname()
	if err != nil {
		e.log.Error(err, "error running controller")
	}

	id = id + "_" + string(uuid.NewUUID())

	lock, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		e.namespace,
		e.name,
		e.kubeClient.CoreV1(),
		e.kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
	)

	if err != nil {
		e.log.Error(err, "error running controller", "namespace", e.namespace, "name", e.name)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				atomic.StoreInt64(&e.isLeader, 1)
				e.log.WithValues("id", id).Info("started leading")
				e.startWork()
			},

			OnStoppedLeading: func() {
				atomic.StoreInt64(&e.isLeader, 0)
				e.log.WithValues("id", id).Info("stopped leading")
				e.stopWork()
			},

			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}

				e.log.WithValues("id", id, "leaderelection", identity).Info("new leaderelection")
			},
		},
	})
}
