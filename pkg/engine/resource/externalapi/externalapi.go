package externalapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/resource/cache"
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
	cache  cache.Cache
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

func New(logger logr.Logger, config apicall.APICallConfiguration) *ExternalAPILoader {
	logger = logger.WithName("external api loader")

	return &ExternalAPILoader{
		logger: logger,
		config: config,
	}
}

func (e *ExternalAPILoader) SetEntries(entries ...*v2alpha1.CachedContextEntry) {
	for _, entry := range entries {
		if entry.Spec.APICall == nil {
			continue
		}
		e.SetEntry(entry)
	}
}

func (e *ExternalAPILoader) SetEntry(entry *v2alpha1.CachedContextEntry) {
	if entry.Spec.APICall == nil {
		return
	}
	rc := entry.Spec.ResourceCache.DeepCopy()
	ctxentry := kyvernov1.ContextEntry{
		Name: entry.Name,
		// Resource: rc,
	}

	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)

	executor, err := apicall.New(e.logger.WithName("apicaller"), nil, ctxentry, nil, nil, e.config)
	if err != nil {
		err := fmt.Errorf("failed to initiaize APICall: %w", err)
		e.logger.Error(err, "")
		_ = e.cache.Set(key, cache.NewInvalidEntry(err))
		return
	}

	interval := time.Duration(entry.Spec.APICall.RefreshIntervalSeconds*int64(time.Nanosecond)) * time.Second
	ticker := time.NewTicker(interval)

	ctx, cancel := context.WithCancel(context.TODO())
	extEntry := &externalEntry{
		logger:    e.logger.WithName("external entry"),
		call:      rc.APICall.APICall.DeepCopy(),
		apicaller: executor,
		ticker:    ticker,
		cancel:    cancel,
	}

	data, err := extEntry.apicaller.Execute(ctx, extEntry.call)
	if err != nil {
		cancel()
		_ = e.cache.Set(key, cache.NewInvalidEntry(err))
		return
	}
	extEntry.data = data

	go extEntry.Poller().Run(ctx, ctx.Done())

	ok := e.cache.Set(key, extEntry.Getter())
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		e.logger.Error(err, "")
		return
	}
	e.logger.V(4).Info("successfully created cache entry", "key", key, "entry", entry)
}

func (e *ExternalAPILoader) Get(rc *kyvernov1.ResourceCache) (interface{}, error) {
	if rc.Resource == nil {
		return nil, fmt.Errorf("resource not found")
	}
	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)
	entry, ok := e.cache.Get(key)
	if !ok {
		err := fmt.Errorf("failed to create fetch entry key=%s", key)
		e.logger.Error(err, "")
		return nil, err
	}
	e.logger.V(4).Info("successfully fetched cache entry")
	return entry.Get()
}

func (e *ExternalAPILoader) Delete(entry *v2alpha1.CachedContextEntry) error {
	if entry.Spec.APICall == nil {
		return fmt.Errorf("invalid object provided")
	}
	rc := entry.Spec.ResourceCache.DeepCopy()
	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)
	ok := e.cache.Delete(key)
	if !ok {
		err := fmt.Errorf("failed to delete ext api loader")
		e.logger.Error(err, "")
		return err
	}
	e.logger.V(4).Info("successfully deleted cache entry")
	return nil
}

func getKeyForExternalEntry(url, caBundle string, interval int64) string {
	return strings.Join([]string{"External= ", url, ", Bundle=", caBundle, "Refresh= ", fmt.Sprint(interval)}, "")
}

type ExternalInformer interface {
	Poller() Poller
	Getter() Getter
}
