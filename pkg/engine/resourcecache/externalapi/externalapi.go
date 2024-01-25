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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	logger               logr.Logger
	call                 *kyvernov1.APICall
	ticker               *time.Ticker
	apicaller            *apicall.APICall
	data                 interface{}
	cancel               context.CancelFunc
	failedFetchesCounter *metric.Int64Counter
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
					metricLabels := []attribute.KeyValue{
						attribute.String("url", string(e.call.Service.URL)),
					}
					(*e.failedFetchesCounter).Add(ctx, 1, metric.WithAttributes(metricLabels...))
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
		err := fmt.Errorf("invalid object provided")
		e.logger.Error(err, "")
		return err
	}
	rc := entry.Spec.ResourceCache.DeepCopy()
	ctxentry := kyvernov1.ContextEntry{
		Name:          entry.Name,
		ResourceCache: rc,
	}
	executor, err := apicall.New(e.logger.WithName("apicaller"), nil, ctxentry, nil, nil, e.config)
	if err != nil {
		err := fmt.Errorf("failed to initiaize APICall: %w", err)
		e.logger.Error(err, "")
		return err
	}

	interval := time.Duration(entry.Spec.APICall.RefreshIntervalSeconds*int64(time.Nanosecond)) * time.Second
	ticker := time.NewTicker(interval)

	ctx, cancel := context.WithCancel(context.Background())
	extEntry := &externalEntry{
		logger:               e.logger.WithName("external entry"),
		call:                 rc.APICall.APICall.DeepCopy(),
		apicaller:            executor,
		ticker:               ticker,
		cancel:               cancel,
		failedFetchesCounter: &e.config.FailedFetchesCounter,
	}

	data, err := extEntry.apicaller.Execute(ctx, extEntry.call)
	if err != nil {
		cancel()
		return err
	}
	extEntry.data = data

	go extEntry.Poller().Run(ctx, ctx.Done())

	key := getKeyForExternalEntry(rc.APICall.Service.URL, rc.APICall.Service.CABundle, rc.APICall.RefreshIntervalSeconds)

	e.logger.V(2).Info("key", key, "entry", entry)
	ok := e.cache.Add(key, extEntry.Getter())
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		e.logger.Error(err, "")
		return err
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
		err := fmt.Errorf("failed to create fetch entry key=%s", key)
		e.logger.Error(err, "")
		return nil, err
	}
	e.logger.V(2).Info("successfully fetched cache entry")
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
