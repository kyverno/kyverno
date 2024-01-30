package externalapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

type Poller interface {
	Run(context.Context, <-chan struct{})
}

type Getter interface {
	Get() (interface{}, error)
	Stop()
}

type ExternalAPILoader struct {
	logger logr.Logger
	config apicall.APICallConfiguration
	store  store.Store
}

type externalEntry struct {
	sync.Mutex
	logger    logr.Logger
	call      *kyvernov1.APICall
	ticker    *time.Ticker
	apicaller *apicall.APICall
	data      interface{}
	cancel    context.CancelFunc
}

func (e *externalEntry) Getter() Getter {
	return e
}

func (e *externalEntry) Poller() Poller {
	return e
}

func (e *externalEntry) Get() (interface{}, error) {
	e.Lock()
	defer e.Unlock()
	e.logger.V(6).Info("cache entry data", "data", e.data)
	return e.data, nil
}

func (e *externalEntry) Stop() {
	e.cancel()
}

func (e *externalEntry) Run(ctx context.Context, stopCh <-chan struct{}) {
	go func() {
		for {
			select {
			case <-e.ticker.C:
				data, err := e.apicaller.Execute(ctx, e.call)
				if err != nil {
					e.logger.Error(err, "failed to get data from api caller")
					return
				}
				e.Lock()
				e.data = data
				e.Unlock()
			case <-stopCh:
				return
			}
		}
	}()
}

func New(logger logr.Logger, config apicall.APICallConfiguration, cache store.Store) *ExternalAPILoader {
	logger = logger.WithName("external api loader")

	return &ExternalAPILoader{
		logger: logger,
		config: config,
		store:  cache,
	}
}

func (e *ExternalAPILoader) SetEntries(entries ...*v2alpha1.GlobalContextEntry) {
	for _, entry := range entries {
		if entry.Spec.APICall == nil {
			continue
		}
		e.SetEntry(entry)
	}
}

func (e *ExternalAPILoader) SetEntry(entry *v2alpha1.GlobalContextEntry) {
	if entry.Spec.APICall == nil {
		return
	}
	rc := entry.Spec.APICall.DeepCopy()
	ctxentry := kyvernov1.ContextEntry{
		Name: entry.Name,
		// Resource: rc,
	}

	key := entry.Name

	jp := jmespath.New(config.NewDefaultConfiguration(false))
	jsonctx := enginecontext.NewContext(jp)

	executor, err := apicall.New(e.logger.WithName("apicaller"), jp, ctxentry, jsonctx, nil, e.config)
	if err != nil {
		err := fmt.Errorf("failed to initiaize APICall: %w", err)
		e.logger.Error(err, "")
		_ = e.store.Set(key, store.NewInvalidEntry(err))
		return
	}

	interval := time.Duration(entry.Spec.APICall.RefreshIntervalSeconds*int64(time.Nanosecond)) * time.Second
	ticker := time.NewTicker(interval)

	ctx, cancel := context.WithCancel(context.TODO())
	extEntry := &externalEntry{
		logger:    e.logger.WithName("external entry"),
		call:      rc.APICall.DeepCopy(),
		apicaller: executor,
		ticker:    ticker,
		cancel:    cancel,
	}

	data, err := extEntry.apicaller.Execute(ctx, extEntry.call)
	if err != nil {
		cancel()
		_ = e.store.Set(key, store.NewInvalidEntry(err))
		return
	}
	extEntry.data = data

	go extEntry.Poller().Run(ctx, ctx.Done())

	ok := e.store.Set(key, extEntry.Getter())
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		e.logger.Error(err, "")
		return
	}
	e.logger.V(4).Info("successfully created cache entry", "key", key, "entry", entry)
}

type ExternalInformer interface {
	Poller() Poller
	Getter() Getter
}
