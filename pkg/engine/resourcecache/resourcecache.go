package resourcecache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache/cache"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache/externalapi"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache/k8sresource"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	k8scache "k8s.io/client-go/tools/cache"
)

type resourceCache struct {
	logger    logr.Logger
	k8sloader *k8sresource.ResourceLoader
	extloader *externalapi.ExternalAPILoader
	jp        jmespath.Interface
	stopch    context.CancelFunc
}

func New(logger logr.Logger, dclient dynamic.Interface, informer v2alpha1.CachedContextEntryInformer, jp jmespath.Interface, config apicall.APICallConfiguration) (Interface, error) {
	logger = logger.WithName("resource cache")
	cacheClient, err := cache.New()
	if err != nil {
		return nil, err
	}

	k8sloader := k8sresource.New(logger, dclient, cacheClient)
	extloader := externalapi.New(logger, config)

	cacheEntryInformer := informer.Informer()

	_, err = cacheEntryInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			entry, ok := obj.(*kyvernov2alpha1.CachedContextEntry)
			if !ok {
				return
			}
			if entry.Spec.IsResource() {
				err := k8sloader.AddEntry(entry)
				if err != nil {
					logger.Error(err, "failed to add entry to k8s resource loader")
					return
				}
			} else if entry.Spec.IsAPICall() {
				err := extloader.AddEntry(entry)
				if err != nil {
					logger.Error(err, "failed to add entry to external api loader")
					return
				}
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newentry, ok := newObj.(*kyvernov2alpha1.CachedContextEntry)
			if !ok {
				return
			}
			oldentry, ok := oldObj.(*kyvernov2alpha1.CachedContextEntry)
			if !ok {
				return
			}

			if reflect.DeepEqual(newentry.Spec, oldentry.Spec) {
				return
			}

			if newentry.Spec.IsResource() {
				err := k8sloader.Delete(oldentry)
				if err != nil {
					logger.Error(err, "failed to delete entry from k8s loader")
					return
				}
				err = k8sloader.AddEntry(newentry)
				if err != nil {
					logger.Error(err, "failed to add entry to k8s loader")
					return
				}
			} else if newentry.Spec.IsAPICall() {
				err := extloader.Delete(oldentry)
				if err != nil {
					logger.Error(err, "failed to delete entry from external api loader")
					return
				}
				err = extloader.AddEntry(newentry)
				if err != nil {
					logger.Error(err, "failed to add entry to external api loader")
					return
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			entry, ok := obj.(*kyvernov2alpha1.CachedContextEntry)
			if !ok {
				return
			}
			if entry.Spec.IsResource() {
				err := k8sloader.Delete(entry)
				if err != nil {
					logger.Error(err, "failed to delete entry from k8s resource loader")
					return
				}
			} else if entry.Spec.IsAPICall() {
				err := extloader.Delete(entry)
				if err != nil {
					logger.Error(err, "failed to delete entry from external api loader")
					return
				}
			}
		},
	})

	if err != nil {
		return nil, err
	}

	entries, err := informer.Lister().List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to fetch context entries")
		return nil, err
	}

	if err := k8sloader.AddEntries(entries...); err != nil {
		logger.Error(err, "failed to add entries to k8s resource loader")
		return nil, err
	}

	if err := extloader.AddEntries(entries...); err != nil {
		logger.Error(err, "failed to add entries to external api loader")
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go cacheEntryInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), cacheEntryInformer.HasSynced) {
		cancel()
		err := errors.New("resource informer cache failed to sync")
		logger.Error(err, "")
		return nil, err
	}

	return &resourceCache{
		logger:    logger,
		k8sloader: k8sloader,
		extloader: extloader,
		jp:        jp,
		stopch:    cancel,
	}, nil
}

func (r *resourceCache) Get(c kyvernov1.ContextEntry, jsonCtx enginecontext.Interface) ([]byte, error) {
	var data interface{}
	var err error
	if c.ResourceCache == nil {
		r.logger.Error(err, "context entry does not have resource cache")
		return nil, fmt.Errorf("resource cache not found")
	}
	rc, err := variables.SubstituteAllInType(r.logger, jsonCtx, c.ResourceCache)
	if err != nil {
		return nil, err
	}
	if rc.Resource != nil {
		if data, err = r.k8sloader.Get(rc); err != nil {
			r.logger.Error(err, "failed to get data from k8sloader")
			return nil, err
		}
	}
	if rc.APICall != nil {
		if data, err = r.extloader.Get(rc); err != nil {
			r.logger.Error(err, "failed to get data from extloader")
			return nil, err
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if c.ResourceCache.JMESPath == "" {
		err := jsonCtx.AddContextEntry(c.Name, jsonData)
		if err != nil {
			r.logger.Error(err, "failed to add resource data to context entry")
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", c.Name, err)
		}

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

	r.logger.V(4).Info("added context data", "name", c.Name, "len", len(contextData))
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
