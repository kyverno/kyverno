package leaderelection

import (
	"context"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"os"
	"time"
)

type Config struct {
	Name       string
	Namespace  string
	StartWork  func()
	StopWork   func()
	KubeClient kubernetes.Interface
	Log        logr.Logger
	isLeader bool
}

func (e *Config) IsLeader() bool {
	return isLeader
}

func (e *Config) Run(ctx context.Context) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id, err := os.Hostname()
	if err != nil {
		e.log.Error(err, "error running controller")
	}

	id = id + "_" + string(uuid.NewUUID())

	var lock = &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      e.name,
			Namespace: e.namespace,
		},
		Client: e.kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				e.log.WithValues("id", id).Info("started leading")
				e.StartWork()
			},

			OnStoppedLeading: func() {
				e.log.WithValues("id", id).Info("stopped leading")
				e.StopWork()
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
