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
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type entry struct {
	sync.RWMutex
	// refreshMu serializes cache refreshes without blocking readers.
	refreshMu   sync.Mutex
	dataMap     map[string]any
	err         error
	stop        func()
	stopOnce    sync.Once
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
	var stopOnce sync.Once
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		stopOnce.Do(func() {
			// Signal the background polling worker (started below via
			// group.StartWithContext) to stop
			cancel()
			// Wait for the group to terminate
			group.Wait()
		})
	}

	projections := make([]store.Projection, 0)
	for _, p := range gce.Spec.Projections {
		if p.Expression != "" {
			prog, errs := compiler.CompileGlobalContextProjection(field.NewPath("expression"), p.Expression)
			if len(errs) > 0 {
				cancel()
				return nil, fmt.Errorf("failed to compile CEL expression for projection %q: %w", p.Name, errs.ToAggregate())
			}
			projections = append(projections, store.Projection{
				Name:    p.Name,
				Program: prog,
			})
			continue
		}
		jpQuery, err := jp.Query(p.JMESPath)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to parse jmespath query: %s", err)
		}
		projections = append(projections, store.Projection{
			Name: p.Name,
			JP:   jpQuery,
		})
	}

	e := &entry{
		dataMap:     make(map[string]any),
		stop:        stop,
		projections: projections,
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		config := apicall.NewAPICallConfiguration(maxResponseLength, apiCallTimeout)
		caller := apicall.NewExecutor(logger, "globalcontext", client, config)

		wait.UntilWithContext(ctx, func(ctx context.Context) {
			if data, err := doCall(ctx, caller, call, gce.Spec.APICall.RetryLimit); err != nil {
				e.setData(ctx, nil, err)

				logger.Error(err, "failed to get data from api caller")

				eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
					APIVersion: gce.APIVersion,
					Kind:       gce.Kind,
					Name:       gce.Name,
					Namespace:  gce.Namespace,
					UID:        gce.UID,
				}, err))
			} else {
				e.setData(ctx, data, nil)

				logger.V(4).Info("api call success", "data", data)

				if shouldUpdateStatus {
					if updateErr := updateStatus(ctx, gce, kyvernoClient); updateErr != nil {
						logger.Error(updateErr, "failed to update status")
					}
				}
			}
		}, period)
	})

	return e, nil
}

func (e *entry) Get(projection string) (any, error) {
	e.RLock()
	defer e.RUnlock()

	if e.err != nil {
		return nil, e.err
	}

	data, ok := e.dataMap[projection]
	if !ok {
		return nil, fmt.Errorf("no data available")
	}

	return data, nil
}

func (e *entry) Stop() {
	e.stopOnce.Do(e.stop)
}

// setData computes the next cache snapshot without blocking policy reads of
// the previous snapshot. The separate refreshMu preserves setData call ordering
// when invoked concurrently, while the RWMutex only protects published state.
func (e *entry) setData(ctx context.Context, data any, fetchErr error) {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	if fetchErr != nil {
		e.setError(fetchErr)
		return
	}

	bytes, ok := data.([]byte)
	if !ok {
		e.setError(fmt.Errorf("data is not a byte array"))
		return
	}

	var jsonData any
	if err := json.Unmarshal(bytes, &jsonData); err != nil {
		e.setError(err)
		return
	}

	// Compute all projections against one private input; an error retains the
	// previous complete snapshot and publishes only the error.
	nextDataMap := make(map[string]any, len(e.projections)+1)
	nextDataMap[""] = jsonData

	// Retain one shared background CEL cost budget per refresh cycle.
	budgetCtx, release := requestbudget.EnsureBackground(ctx)
	defer release()

	for _, projection := range e.projections {
		result, err := projection.Evaluate(budgetCtx, jsonData)
		if err != nil {
			if projection.Program != nil {
				err = fmt.Errorf("failed to apply projection %q: %w (object is the parsed API response)", projection.Name, err)
			}
			e.setError(err)
			return
		}
		nextDataMap[projection.Name] = result
	}

	// Readers observe either the complete old or the complete new snapshot.
	e.Lock()
	e.dataMap = nextDataMap
	e.err = nil
	e.Unlock()
}

// setError preserves the last good snapshot but retains the original
// behavior of rejecting reads until a later successful refresh.
func (e *entry) setError(err error) {
	e.Lock()
	e.err = err
	e.Unlock()
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
		return err != nil
	}, func() error {
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
