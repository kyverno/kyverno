package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/externalapi"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/k8sresource"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/client-go/dynamic"
	k8scache "k8s.io/client-go/tools/cache"
)

type resourceCache struct {
	logger    logr.Logger
	k8sloader *k8sresource.ResourceLoader
	extloader *externalapi.ExternalAPILoader
	jp        jmespath.Interface
	stopch    context.CancelFunc
	store     store.Store
}

func New(logger logr.Logger, dclient dynamic.Interface, informer v2alpha1.GlobalContextEntryInformer, jp jmespath.Interface, config apicall.APICallConfiguration) (Interface, error) {
	logger = logger.WithName("resource cache")

	storage := store.New()
	k8sloader := k8sresource.New(logger, dclient, storage)
	extloader := externalapi.New(logger, config, storage)

	cacheEntryInformer := informer.Informer()
	ctx, cancel := context.WithCancel(context.TODO())
	go cacheEntryInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), cacheEntryInformer.HasSynced) {
		cancel()
		err := errors.New("resource informer cache failed to sync")
		logger.Error(err, "")
		return nil, err
	}

	ccEventHandler, err := cacheEntryInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			logger.V(4).Info("new cached context entry added, creating cache entry")
			entry, ok := obj.(*kyvernov2alpha1.GlobalContextEntry)
			if !ok {
				return
			}
			if entry.Spec.IsResource() {
				k8sloader.SetEntry(entry)
			} else if entry.Spec.IsAPICall() {
				extloader.SetEntry(entry)
			}
		},
		UpdateFunc: func(_ interface{}, newObj interface{}) {
			logger.V(4).Info("cached context entry updated, updating cache entry")
			newentry, ok := newObj.(*kyvernov2alpha1.GlobalContextEntry)
			if !ok {
				return
			}

			if newentry.Spec.IsResource() {
				k8sloader.SetEntry(newentry)
			} else if newentry.Spec.IsAPICall() {
				extloader.SetEntry(newentry)
			}
		},
		DeleteFunc: func(obj interface{}) {
			logger.V(4).Info("cached context entry deleted, deleting cache entry")
			entry, ok := obj.(*kyvernov2alpha1.GlobalContextEntry)
			if !ok {
				return
			}
			key := entry.Name
			storage.Delete(key)
		},
	})
	if err != nil {
		cancel()
		return nil, err
	}

	if !k8scache.WaitForCacheSync(ctx.Done(), ccEventHandler.HasSynced) {
		cancel()
		err := errors.New("resource informer cache event handler failed to sync")
		logger.Error(err, "")
		return nil, err
	}

	return &resourceCache{
		logger:    logger,
		k8sloader: k8sloader,
		extloader: extloader,
		jp:        jp,
		stopch:    cancel,
		store:     storage,
	}, nil
}

func (r *resourceCache) Get(c ContextEntry, jsonCtx enginecontext.Interface) ([]byte, error) {
	var data interface{}
	var err error
	if c.Resource == nil {
		r.logger.Error(err, "context entry does not have resource cache")
		return nil, fmt.Errorf("resource cache not found")
	}
	rc, err := variables.SubstituteAllInType(r.logger, jsonCtx, c.Resource)
	if err != nil {
		return nil, err
	}
	r.logger.V(6).Info("variables substituted", "resource", rc)

	key := c.Resource.Name
	entry, ok := r.store.Get(key)
	if !ok {
		err := fmt.Errorf("failed to fetch entry key=%s", key)
		r.logger.Error(err, "")
		return nil, err
	}
	r.logger.V(4).Info("successfully fetched cache entry", "key", key, "entry", entry)
	data, err = entry.Get()
	if data, err = entry.Get(); err != nil {
		r.logger.Error(err, "failed to get data from global context")
		return nil, err
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	r.logger.V(6).Info("fetched json data", "name", c.Name, "jsondata", jsonData)

	if c.Resource.JMESPath == "" {
		err := jsonCtx.AddContextEntry(c.Name, jsonData)
		if err != nil {
			r.logger.Error(err, "failed to add resource data to context entry")
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", c.Name, err)
		}

		r.logger.V(6).Info("added context data", "name", c.Name, "contextData", jsonData)
		return jsonData, nil
	}

	path, err := variables.SubstituteAll(r.logger, jsonCtx, rc.JMESPath)
	if err != nil {
		r.logger.Error(err, "failed to substitute variables in context entry")
		return nil, fmt.Errorf("failed to substitute variables in context entry %s JMESPath %s: %w", c.Name, rc.JMESPath, err)
	}

	results, err := r.applyJMESPathJSON(path.(string), jsonData)
	if err != nil {
		r.logger.Error(err, "failed to apply JMESPath for context entry")
		return nil, fmt.Errorf("failed to apply JMESPath %s for context entry %s: %w", path, c.Name, err)
	}
	r.logger.V(6).Info("applied jmespath expression", "name", c.Name, "results", results)

	contextData, err := json.Marshal(results)
	if err != nil {
		r.logger.Error(err, "failed to marshal APICall data for context entry")
		return nil, fmt.Errorf("failed to marshal APICall data for context entry %s: %w", c.Name, err)
	}

	err = jsonCtx.AddContextEntry(c.Name, contextData)
	if err != nil {
		r.logger.Error(err, "failed to add resource cache results for context entry")
		return nil, fmt.Errorf("failed to add resource cache results for context entry %s: %w", c.Name, err)
	}

	r.logger.V(6).Info("added context data", "name", c.Name, "contextData", contextData)
	return contextData, nil
}

func (r *resourceCache) applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(jsonData), err)
	}
	return r.jp.Search(jmesPath, data)
}
