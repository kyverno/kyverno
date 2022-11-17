package resource

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	pkgutils "github.com/kyverno/kyverno/pkg/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

type EventHandler func(types.UID, schema.GroupVersionKind, Resource)

type MetadataCache interface {
	GetResourceHash(uid types.UID) (Resource, schema.GroupVersionKind, bool)
	AddEventHandler(EventHandler)
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

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
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

func (c *controller) AddEventHandler(eventHandler EventHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.eventHandlers = append(c.eventHandlers, eventHandler)
	for _, watcher := range c.dynamicWatchers {
		for uid, resource := range watcher.hashes {
			eventHandler(uid, watcher.gvk, resource)
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
	policies, err := c.fetchPolicies(logger, metav1.NamespaceAll)
	if err != nil {
		return err
	}
	kinds := utils.BuildKindSet(logger, utils.RemoveNonValidationPolicies(logger, append(clusterPolicies, policies...)...)...)
	gvrs := map[schema.GroupVersionKind]schema.GroupVersionResource{}
	for _, kind := range kinds.List() {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		apiResource, gvr, err := c.client.Discovery().FindResource(apiVersion, kind)
		if err != nil {
			logger.Error(err, "failed to get gvr from kind", "kind", kind)
		} else {
			gvk := schema.GroupVersionKind{Group: apiResource.Group, Version: apiResource.Version, Kind: apiResource.Kind}
			if !reportutils.IsGvkSupported(gvk) {
				logger.Info("kind is not supported", "gvk", gvk)
			} else {
				if pkgutils.ContainsString(apiResource.Verbs, "list") && pkgutils.ContainsString(apiResource.Verbs, "watch") {
					gvrs[gvk] = gvr
				} else {
					logger.Info("list/watch not supported for kind", "kind", kind)
				}
			}
		}
	}
	dynamicWatchers := map[schema.GroupVersionResource]*watcher{}
	for gvk, gvr := range gvrs {
		// if we already have one, transfer it to the new map
		if c.dynamicWatchers[gvr] != nil {
			dynamicWatchers[gvr] = c.dynamicWatchers[gvr]
			delete(c.dynamicWatchers, gvr)
		} else {
			hashes := map[types.UID]Resource{}
			objs, err := c.client.GetDynamicInterface().Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				logger.Error(err, "failed to list resources", "gvr", gvr)
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
					c.notify(uid, gvk, hashes[uid])
				}
				logger.Info("start watcher ...", "gvr", gvr, "resourceVersion", resourceVersion)

				watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
					return c.client.GetDynamicInterface().Resource(gvr).Watch(ctx, options)
				}
				watchInterface, err := watchTools.NewRetryWatcher(resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
				if err != nil {
					logger.Error(err, "failed to create watcher", "gvr", gvr)
				} else {
					w := &watcher{
						watcher: watchInterface,
						gvk:     gvk,
						hashes:  hashes,
					}
					go func() {
						gvr := gvr
						defer logger.Info("watcher stopped")
						for event := range watchInterface.ResultChan() {
							switch event.Type {
							case watch.Added:
								c.updateHash(event.Object.(*unstructured.Unstructured), gvr)
							case watch.Modified:
								c.updateHash(event.Object.(*unstructured.Unstructured), gvr)
							case watch.Deleted:
								c.deleteHash(event.Object.(*unstructured.Unstructured), gvr)
							}
						}
					}()
					dynamicWatchers[gvr] = w
				}
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
			c.notify(uid, watcher.gvk, resource)
		}
	}
	return nil
}

func (c *controller) notify(uid types.UID, gvk schema.GroupVersionKind, obj Resource) {
	for _, handler := range c.eventHandlers {
		handler(uid, gvk, obj)
	}
}

func (c *controller) updateHash(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
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
			c.notify(uid, watcher.gvk, watcher.hashes[uid])
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
		c.notify(uid, watcher.gvk, hash)
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

func (c *controller) fetchPolicies(logger logr.Logger, namespace string) ([]kyvernov1.PolicyInterface, error) {
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
