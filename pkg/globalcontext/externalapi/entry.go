package externalapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	publishMu   sync.Mutex // serializes shutdown with data, status, and event publication
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
	call kyvernov1.APICall,
	period time.Duration,
	maxResponseLength int64,
	apiCallTimeout time.Duration,
	shouldUpdateStatus bool,
	jp jmespath.Interface,
) (store.Entry, error) {
	var group wait.Group
	ctx, cancel := context.WithCancel(ctx)

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

	e := &entry{
		dataMap:     make(map[string]any),
		projections: projections,
	}
	// Cancel under the same lock used for publication. A completed poll must
	// never publish after teardown has marked the entry stopped.
	e.stop = func() {
		e.publishMu.Lock()
		e.stopped = true
		cancel()
		e.publishMu.Unlock()
		// Never wait while holding publishMu: the worker may need that lock.
		group.Wait()
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		config := apicall.NewAPICallConfiguration(maxResponseLength, apiCallTimeout)
		caller := apicall.NewExecutor(logger, "globalcontext", client, config)

		wait.UntilWithContext(ctx, func(ctx context.Context) {
			data, err := doCall(ctx, caller, call, gce.Spec.APICall.RetryLimit)
			// Check cancellation and publish atomically with entry shutdown.
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

					if shouldUpdateStatus {
						if updateErr := updateStatus(ctx, gce, kyvernoClient); updateErr != nil {
							logger.Error(updateErr, "failed to update status")
						}
					}
				}
			})
		}, period)
	})

	return e, nil
}

// publish serializes the final cancellation check with every poll side effect.
// The callback must not call Stop; Stop waits for the worker outside the lock.
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

func doCall(ctx context.Context, caller apicall.Executor, call kyvernov1.APICall, retryLimit int) (any, error) {
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
