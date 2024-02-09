package externalapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/event"
	entryevent "github.com/kyverno/kyverno/pkg/globalcontext/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
	logger logr.Logger,
	client apicall.ClientInterface,
	call kyvernov1.APICall,
	period time.Duration,
	maxResponseLength int64,
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

	var wg sync.WaitGroup
	wg.Add(1)

	group.StartWithContext(ctx, func(ctx context.Context) {
		defer wg.Done()

		config := apicall.NewAPICallConfiguration(maxResponseLength)
		caller := apicall.NewCaller(logger, "globalcontext", client, config)
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			if data, err := doCall(ctx, caller, call); err != nil {
				logger.Error(err, "failed to get data from api caller")
				gce.Status.SetReady(false, err.Error())
				eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
					APIVersion: gce.APIVersion,
					Kind:       gce.Kind,
					Name:       gce.Name,
					Namespace:  gce.Namespace,
					UID:        gce.UID,
				}, entryevent.ReasonAPICallFailure, err))
				e.setData(nil, err)
			} else {
				gce.Status.SetReady(true, "Data fetched successfully")
				e.setData(data, nil)
			}
		}, period)
	})

	wg.Wait()

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
