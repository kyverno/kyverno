package externalapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
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
	dataMap     map[string]any
	err         error
	stop        func()
	stopOnce    sync.Once
	publishMu   sync.Mutex // serializes shutdown with cache and event publication
	statusMu    sync.Mutex // serializes status refreshes with completed shutdown
	stopped     bool       // guarded by publishMu
	projections []store.Projection
}

func New(
	ctx context.Context,
	gce *kyvernov2beta1.GlobalContextEntry,
	eventGen event.Interface,
	kyvernoClient versioned.Interface,
	gceLister kyvernov2beta1listers.GlobalContextEntryLister,
	logger logr.Logger,
	client apicall.ClientInterface,
	call kyverno.APICall,
	period time.Duration,
	maxResponseLength int64,
	apiCallTimeout time.Duration,
	shouldUpdateStatus bool,
	jp jmespath.Interface,
) (store.Entry, error) {
	var group wait.Group

	projections := make([]store.Projection, 0)
	for _, p := range gce.Spec.Projections {
		jpQuery, err := jp.Query(p.JMESPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse jmespath query: %s", err)
		}
		projections = append(projections, store.Projection{
			Name: p.Name,
			JP:   jpQuery,
		})
	}

	// Establish the poll context only after validation succeeds, so failed
	// projection compilation does not leak a child context.
	ctx, cancel := context.WithCancel(ctx)
	e := &entry{
		dataMap:     make(map[string]any),
		projections: projections,
	}
	// Cancel before waiting for any in-flight I/O. statusMu prevents a
	// refresh from surviving the transition to stopped; publishMu guards
	// cache and event publication. Neither gate blocks cancellation itself.
	e.stop = func() {
		cancel()
		e.statusMu.Lock()
		e.publishMu.Lock()
		e.stopped = true
		e.publishMu.Unlock()
		e.statusMu.Unlock()
		// Wait only after releasing both locks to avoid worker deadlocks.
		group.Wait()
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		config := apicall.NewAPICallConfiguration(maxResponseLength, apiCallTimeout)
		caller := apicall.NewExecutor(logger, "globalcontext", client, config)

		wait.UntilWithContext(ctx, func(ctx context.Context) {
			data, err := doCall(ctx, caller, call, gce.Spec.APICall.RetryLimit)
			// Publish cache updates and events behind the shutdown gate.
			// Kubernetes status I/O must not run while holding publishMu:
			// Stop needs to cancel an in-flight status request.
			e.publish(ctx, func() {
				if err != nil {
					e.setData(nil, err)

					logger.Error(err, "failed to get data from api caller")

					eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
						APIVersion: gce.APIVersion,
						Kind:       gce.Kind,
						Name:       gce.Name,
						Namespace:  gce.Namespace,
						UID:        gce.UID,
					}, err))
				} else {
					e.setData(data, nil)

					logger.V(4).Info("api call success", "data", data)
				}
			})

			// Status publication uses a separate gate from cache/events.
			// Stop cancels the context before waiting for statusMu, so an
			// in-flight Kubernetes request can exit on cancellation.
			if err == nil && shouldUpdateStatus {
				e.refreshStatus(ctx, gce, kyvernoClient, logger)
			}
		}, period)
	})

	return e, nil
}

// refreshStatus prevents a status update from outliving the stopped state.
// Do not acquire publishMu while holding statusMu: Stop acquires statusMu
// before marking stopped under publishMu.
func (e *entry) refreshStatus(
	ctx context.Context,
	gce *kyvernov2beta1.GlobalContextEntry,
	kyvernoClient versioned.Interface,
	logger logr.Logger,
) {
	e.statusMu.Lock()
	defer e.statusMu.Unlock()
	if ctx.Err() != nil {
		return
	}
	if err := updateStatus(ctx, gce, kyvernoClient); err != nil && ctx.Err() == nil {
		logger.Error(err, "failed to update status")
	}
}

// publish guards cache and event publication against completed shutdown.
// Callbacks must not perform blocking Kubernetes I/O or call Stop.
func (e *entry) publish(ctx context.Context, fn func()) {
	e.publishMu.Lock()
	defer e.publishMu.Unlock()
	if e.stopped || ctx.Err() != nil {
		return
	}
	fn()
}

func (e *entry) Get(projection string) (any, error) {
	e.Lock()
	defer e.Unlock()

	if e.err != nil {
		return nil, e.err
	}

	data := e.dataMap[projection]
	if data == nil {
		return nil, fmt.Errorf("no data available")
	}

	return data, nil
}

func (e *entry) Stop() {
	e.stopOnce.Do(e.stop)
}

func (e *entry) setData(data any, err error) {
	e.Lock()
	defer e.Unlock()

	if err != nil {
		e.err = err
	} else {
		var jsonData any
		if bytes, ok := data.([]byte); ok {
			err = json.Unmarshal(bytes, &jsonData)
			if err != nil {
				e.err = err
				return
			}
		} else {
			e.err = fmt.Errorf("data is not a byte array")
			return
		}
		newDataMap := make(map[string]any)
		newDataMap[""] = jsonData
		for _, projection := range e.projections {
			result, err := projection.JP.Search(jsonData)
			if err != nil {
				e.err = err
				return
			}
			newDataMap[projection.Name] = result
		}
		e.dataMap = newDataMap
		e.err = nil
	}
}

func doCall(ctx context.Context, caller apicall.Executor, call kyverno.APICall, retryLimit int) (any, error) {
	var result any
	backoff := wait.Backoff{
		Duration: retry.DefaultBackoff.Duration,
		Factor:   retry.DefaultBackoff.Factor,
		Jitter:   retry.DefaultBackoff.Jitter,
		Steps:    retryLimit,
	}

	retryError := retry.OnError(backoff, func(err error) bool {
		// Do not retry cancellation failures during entry teardown.
		return err != nil && ctx.Err() == nil
	}, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var exeErr error
		result, exeErr = caller.Execute(ctx, &call)
		return exeErr
	})

	return result, retryError
}

func updateStatus(ctx context.Context, gce *kyvernov2beta1.GlobalContextEntry, kyvernoClient versioned.Interface) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the latest version of the GlobalContextEntry
		latest, err := kyvernoClient.KyvernoV2beta1().GlobalContextEntries().Get(ctx, gce.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		return controllerutils.UpdateStatus(ctx, latest, kyvernoClient.KyvernoV2beta1().GlobalContextEntries(), func(latest *kyvernov2beta1.GlobalContextEntry) error {
			if latest == nil {
				return fmt.Errorf("failed to update status: %s", gce.GetName())
			}
			latest.Status.UpdateRefreshTime()
			return nil
		}, nil)
	})

	return retryErr
}
