package resource

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchTools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "resource-report-controller"
	maxRetries     = 5
)

type Resource struct {
	Namespace string
	Name      string
	Hash      string
}

type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Stopped  EventType = "STOPPED"
)

type EventHandler func(EventType, types.UID, schema.GroupVersionKind, Resource)

type MetadataCache interface {
	GetResourceHash(uid types.UID) (Resource, schema.GroupVersionKind, bool)
	GetAllResourceKeys() []string
	AddEventHandler(EventHandler)
	Warmup(ctx context.Context) error
}

type Controller interface {
	controllers.Controller
	MetadataCache
}

type watcher struct {
	watcher watch.Interface
	gvk     schema.GroupVersionKind
	hashes  map[types.UID]Resource
}

type controller struct {
	// clients
	client dclient.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister

	// queue
	queue workqueue.RateLimitingInterface

	lock            sync.RWMutex
	dynamicWatchers map[schema.GroupVersionResource]*watcher
	eventHandlers   []EventHandler
}

func NewController(
	client dclient.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
) Controller {
	c := controller{
		client:          client,
		polLister:       polInformer.Lister(),
		cpolLister:      cpolInformer.Lister(),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
	}
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Warmup(ctx context.Context) error {
	return c.updateDynamicWatchers(ctx)
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
	c.stopDynamicWatchers()
}

func (c *controller) GetResourceHash(uid types.UID) (Resource, schema.GroupVersionKind, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, watcher := range c.dynamicWatchers {
		if resource, exists := watcher.hashes[uid]; exists {
			return resource, watcher.gvk, true
		}
	}
	return Resource{}, schema.GroupVersionKind{}, false
}

func (c *controller) GetAllResourceKeys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	var keys []string
	for _, watcher := range c.dynamicWatchers {
		for uid, resource := range watcher.hashes {
			key := string(uid)
			if resource.Namespace != "" {
				key = resource.Namespace + "/" + key
			}
			keys = append(keys, key)
		}
	}
	return keys
}

func (c *controller) AddEventHandler(eventHandler EventHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.eventHandlers = append(c.eventHandlers, eventHandler)
	for _, watcher := range c.dynamicWatchers {
		for uid, resource := range watcher.hashes {
			eventHandler(Added, uid, watcher.gvk, resource)
		}
	}
}

func (c *controller) startWatcher(ctx context.Context, logger logr.Logger, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) (*watcher, error) {
	hashes := map[types.UID]Resource{}
	objs, err := c.client.GetDynamicInterface().Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "failed to list resources")
		return nil, err
	} else {
		resourceVersion := objs.GetResourceVersion()
		for _, obj := range objs.Items {
			uid := obj.GetUID()
			hash := reportutils.CalculateResourceHash(obj)
			hashes[uid] = Resource{
				Hash:      hash,
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			}
			c.notify(Added, uid, gvk, hashes[uid])
		}
		logger := logger.WithValues("resourceVersion", resourceVersion)
		logger.Info("start watcher ...")
		watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
			logger.Info("creating watcher...")
			watch, err := c.client.GetDynamicInterface().Resource(gvr).Watch(context.Background(), options)
			if err != nil {
				logger.Error(err, "failed to watch")
			}
			return watch, err
		}
		watchInterface, err := watchTools.NewRetryWatcher(resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
		if err != nil {
			logger.Error(err, "failed to create watcher")
			return nil, err
		} else {
			w := &watcher{
				watcher: watchInterface,
				gvk:     gvk,
				hashes:  hashes,
			}
			go func(gvr schema.GroupVersionResource) {
				defer logger.Info("watcher stopped")
				for event := range watchInterface.ResultChan() {
					switch event.Type {
					case watch.Added:
						c.updateHash(Added, event.Object.(*unstructured.Unstructured), gvr)
					case watch.Modified:
						c.updateHash(Modified, event.Object.(*unstructured.Unstructured), gvr)
					case watch.Deleted:
						c.deleteHash(event.Object.(*unstructured.Unstructured), gvr)
					case watch.Error:
						logger.Error(errors.New("watch error event received"), "watch error event received", "event", event.Object)
					}
				}
			}(gvr)
			return w, nil
		}
	}
}

func (c *controller) updateDynamicWatchers(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	clusterPolicies, err := c.fetchClusterPolicies(logger)
	if err != nil {
		return err
	}
	policies, err := c.fetchPolicies(metav1.NamespaceAll)
	if err != nil {
		return err
	}
	kinds := utils.BuildKindSet(logger, utils.RemoveNonValidationPolicies(append(clusterPolicies, policies...)...)...)
	gvkToGvr := map[schema.GroupVersionKind]schema.GroupVersionResource{}
	for _, policyKind := range sets.List(kinds) {
		group, version, kind, subresource := kubeutils.ParseKindSelector(policyKind)
		gvrss, err := c.client.Discovery().FindResources(group, version, kind, subresource)
		if err != nil {
			logger.Error(err, "failed to get gvr from kind", "kind", kind)
		} else {
			for gvrs, api := range gvrss {
				if gvrs.SubResource == "" {
					gvk := schema.GroupVersionKind{Group: gvrs.Group, Version: gvrs.Version, Kind: policyKind}
					if !reportutils.IsGvkSupported(gvk) {
						logger.Info("kind is not supported", "gvk", gvk)
					} else {
						if slices.Contains(api.Verbs, "list") && slices.Contains(api.Verbs, "watch") {
							gvkToGvr[gvk] = gvrs.GroupVersionResource()
						} else {
							logger.Info("list/watch not supported for kind", "kind", kind)
						}
					}
				}
			}
		}
	}
	dynamicWatchers := map[schema.GroupVersionResource]*watcher{}
	for gvk, gvr := range gvkToGvr {
		logger := logger.WithValues("gvr", gvr, "gvk", gvk)
		// if we already have one, transfer it to the new map
		if c.dynamicWatchers[gvr] != nil {
			dynamicWatchers[gvr] = c.dynamicWatchers[gvr]
			delete(c.dynamicWatchers, gvr)
		} else {
			if w, err := c.startWatcher(ctx, logger, gvr, gvk); err != nil {
				logger.Error(err, "failed to start watcher")
			} else {
				dynamicWatchers[gvr] = w
			}
		}
	}
	oldDynamicWatcher := c.dynamicWatchers
	c.dynamicWatchers = dynamicWatchers
	// shutdown remaining watcher
	for gvr, watcher := range oldDynamicWatcher {
		watcher.watcher.Stop()
		delete(oldDynamicWatcher, gvr)
		for uid, resource := range watcher.hashes {
			c.notify(Stopped, uid, watcher.gvk, resource)
		}
	}
	return nil
}

func (c *controller) stopDynamicWatchers() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, watcher := range c.dynamicWatchers {
		watcher.watcher.Stop()
	}
	c.dynamicWatchers = map[schema.GroupVersionResource]*watcher{}
}

func (c *controller) notify(eventType EventType, uid types.UID, gvk schema.GroupVersionKind, obj Resource) {
	for _, handler := range c.eventHandlers {
		handler(eventType, uid, gvk, obj)
	}
}

func (c *controller) updateHash(eventType EventType, obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	c.lock.Lock()
	defer c.lock.Unlock()
	watcher, exists := c.dynamicWatchers[gvr]
	if exists {
		uid := obj.GetUID()
		hash := reportutils.CalculateResourceHash(*obj)
		if exists && hash != watcher.hashes[uid].Hash {
			watcher.hashes[uid] = Resource{
				Hash:      hash,
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			}
			c.notify(eventType, uid, watcher.gvk, watcher.hashes[uid])
		}
	}
}

func (c *controller) deleteHash(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	c.lock.Lock()
	defer c.lock.Unlock()
	watcher, exists := c.dynamicWatchers[gvr]
	if exists {
		uid := obj.GetUID()
		hash := watcher.hashes[uid]
		delete(watcher.hashes, uid)
		c.notify(Deleted, uid, watcher.gvk, hash)
	}
}

func (c *controller) fetchClusterPolicies(logger logr.Logger) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			policies = append(policies, cpol)
		}
	}
	return policies, nil
}

func (c *controller) fetchPolicies(namespace string) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := c.polLister.Policies(namespace).List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, pol)
		}
	}
	return policies, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	return c.updateDynamicWatchers(ctx)
}
