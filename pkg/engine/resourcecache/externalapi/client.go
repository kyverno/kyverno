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
	"github.com/kyverno/kyverno/pkg/engine/resourcecache/cache"
)

type Poller interface {
	Run(stopCh <-chan struct{})
}

type Getter interface {
	Get() (interface{}, error)
}

type ExternalAPILoader struct {
	logger logr.Logger
	config apicall.APICallConfiguration
	cache  cache.Cache
}

type externalEntry struct {
	sync.Mutex
	ctx       context.Context
	call      *kyvernov1.APICall
	ticker    *time.Ticker
	apicaller *apicall.APICall
	data      interface{}
}

func New(logger logr.Logger, config apicall.APICallConfiguration) *ExternalAPILoader {
	return &ExternalAPILoader{
		logger: logger,
		config: config,
	}
}

func (e *ExternalAPILoader) AddEntries(entries ...*v2alpha1.CachedContextEntry) error {
	for _, entry := range entries {
		if entry.Spec.APICall == nil {
			continue
		}
		err := e.AddEntry(entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *ExternalAPILoader) AddEntry(entry *v2alpha1.CachedContextEntry) error {
	if entry.Spec.APICall == nil {
		return fmt.Errorf("Invalid object provided")
	}
	rc := entry.Spec.ResourceCache.DeepCopy()
	ctxentry := kyvernov1.ContextEntry{
		Name:          entry.Name,
		ResourceCache: rc,
	}
	executor, err := apicall.New(e.logger, nil, ctxentry, nil, nil, e.config)
	if err != nil {
		return fmt.Errorf("failed to initiaize APICall: %w", err)
	}

	interval := time.Duration(entry.Spec.APICall.RefreshIntervalSeconds*int64(time.Nanosecond)) * time.Second
	ticker := time.NewTicker(interval)

	extEntry := &externalEntry{
		ctx:       context.Background(),
		call:      rc.APICall.APICall.DeepCopy(),
		apicaller: executor,
		ticker:    ticker,
	}

	data, err := extEntry.apicaller.Execute(extEntry.ctx, extEntry.call)
	if err != nil {
		return err
	}
	extEntry.data = data

	ctx, cancel := context.WithCancel(context.Background())
	go extEntry.Poller().Run(ctx.Done())

	cacheEntry := &cache.CacheEntry{
		Entry: extEntry.Getter(),
		Stop:  cancel,
	}
	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)

	e.logger.V(2).Info("key", key, "entry", entry)
	ok := e.cache.Add(key, cacheEntry)
	if !ok {
		return fmt.Errorf("failed to create cache entry key=%s", key)
	}
	e.logger.V(2).Info("successfully created cache entry")
	return nil
}

func (e *ExternalAPILoader) Get(rc *kyvernov1.ResourceCache) (interface{}, error) {
	if rc.Resource == nil {
		return nil, fmt.Errorf("resource not found")
	}
	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)
	entry, ok := e.cache.Get(key)
	if !ok {
		return nil, fmt.Errorf("failed to create fetch entry key=%s", key)
	}
	e.logger.V(2).Info("successfully fetched cache entry")
	return entry.Get()
}

func (e *ExternalAPILoader) Delete(entry *v2alpha1.CachedContextEntry) error {
	if entry.Spec.APICall == nil {
		return fmt.Errorf("Invalid object provided")
	}
	rc := entry.Spec.ResourceCache.DeepCopy()
	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)
	ok := e.cache.Delete(key)
	if !ok {
		return fmt.Errorf("failed to delete ext api loader")
	}
	e.logger.V(2).Info("successfully deleted cache entry")
	return nil
}

func getKeyForExternalEntry(url, caBundle string, interval int64) string {
	return strings.Join([]string{"External= ", url, ", Bundle=", caBundle, "Refresh= ", fmt.Sprint(interval)}, "")
}

type ExternalInformer interface {
	Poller() Poller
	Getter() Getter
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
	return e.data, nil
}

func (e *externalEntry) Run(stopCh <-chan struct{}) {
	go func() {
		for {
			select {
			case <-e.ticker.C:
				data, err := e.apicaller.Execute(e.ctx, e.call)
				if err != nil {
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
