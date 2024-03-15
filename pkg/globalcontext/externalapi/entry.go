package externalapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/event"
	entryevent "github.com/kyverno/kyverno/pkg/globalcontext/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type entry struct {
	sync.Mutex
	data any
	err  error
	stop func()
}

func New(
	ctx context.Context,
	gce *kyvernov2alpha1.GlobalContextEntry,
	eventGen event.Interface,
	kyvernoClient versioned.Interface,
	gceLister kyvernov2alpha1listers.GlobalContextEntryLister,
	logger logr.Logger,
	client apicall.ClientInterface,
	call kyvernov1.APICall,
	period time.Duration,
	maxResponseLength int64,
	shouldUpdateStatus bool,
) (store.Entry, error) {
	var group wait.Group
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		// Send stop signal to informer's goroutine
		cancel()
		// Wait for the group to terminate
		group.Wait()
	}
	e := &entry{
		stop: stop,
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		config := apicall.NewAPICallConfiguration(maxResponseLength)
		caller := apicall.NewCaller(logger, "globalcontext", client, config)

		wait.UntilWithContext(ctx, func(ctx context.Context) {
			if data, err := doCall(ctx, caller, call); err != nil {
				e.setData(nil, err)

				logger.Error(err, "failed to get data from api caller")

				eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
					APIVersion: gce.APIVersion,
					Kind:       gce.Kind,
					Name:       gce.Name,
					Namespace:  gce.Namespace,
					UID:        gce.UID,
				}, entryevent.ReasonAPICallFailure, err))

				if shouldUpdateStatus {
					if updateErr := updateStatus(ctx, gce.Name, kyvernoClient, false, entryevent.ReasonAPICallFailure); updateErr != nil {
						logger.Error(updateErr, "failed to update status")
					}
				}
			} else {
				e.setData(data, nil)

				logger.V(4).Info("api call success", "data", data)

				if shouldUpdateStatus {
					if updateErr := updateStatus(ctx, gce.Name, kyvernoClient, true, "APICallSuccess"); updateErr != nil {
						logger.Error(updateErr, "failed to update status")
					}
				}
			}
		}, period)
	})

	return e, nil
}

func (e *entry) Get() (any, error) {
	e.Lock()
	defer e.Unlock()

	if e.err != nil {
		return nil, e.err
	}

	if e.data == nil {
		return nil, fmt.Errorf("no data available")
	}

	return e.data, nil
}

func (e *entry) Stop() {
	e.Lock()
	defer e.Unlock()
	e.stop()
}

func (e *entry) setData(data any, err error) {
	e.Lock()
	defer e.Unlock()

	if err != nil {
		e.err = err
	} else {
		e.data = data
	}
}

func doCall(ctx context.Context, caller apicall.Caller, call kyvernov1.APICall) (any, error) {
	return caller.Execute(ctx, &call)
}

func updateStatus(ctx context.Context, gceName string, kyvernoClient versioned.Interface, ready bool, reason string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestGCE, getErr := kyvernoClient.KyvernoV2alpha1().GlobalContextEntries().Get(ctx, gceName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		_, updateErr := controllerutils.UpdateStatus(ctx, latestGCE, kyvernoClient.KyvernoV2alpha1().GlobalContextEntries(), func(latest *kyvernov2alpha1.GlobalContextEntry) error {
			if latest == nil {
				return fmt.Errorf("failed to update status: %s", latestGCE.Name)
			}
			latest.Status.SetReady(ready, reason)
			if ready {
				latest.Status.UpdateRefreshTime()
			}
			return nil
		})

		return updateErr
	})

	return retryErr
}
