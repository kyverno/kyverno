package externalapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
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
	logger logr.Logger,
	client apicall.ClientInterface,
	call kyvernov1.APICall,
	period time.Duration,
	maxResponseLength int64,
) (*entry, error) {
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
		// TODO: make sure we have called it at least once before returning
		config := apicall.NewAPICallConfiguration(maxResponseLength)
		caller := apicall.NewCaller(logger, "globalcontext", client, config)
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			if data, err := doCall(ctx, caller, call); err != nil {
				logger.Error(err, "failed to get data from api caller")
				e.setData(nil, err)
			} else {
				e.setData(data, nil)
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
