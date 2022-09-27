package resource

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 5
	workers    = 1
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
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
	}
	controllerutils.AddDefaultEventHandlers(logger.V(3), polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger.V(3), cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger.V(3), c.queue, workers, maxRetries, c.reconcile, stopCh)
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

func (c *controller) updateDynamicWatchers() error {
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
	kinds := utils.BuildKindSet(logger, utils.RemoveNonBackgroundPolicies(logger, append(clusterPolicies, policies...)...)...)
	gvrs := map[string]schema.GroupVersionResource{}
	for _, kind := range kinds.List() {
		gvr, err := c.client.Discovery().GetGVRFromKind(kind)
		if err == nil {
			gvrs[kind] = gvr
		} else {
			logger.Error(err, "failed to get gvr from kind", "kind", kind)
		}
	}
	dynamicWatchers := map[schema.GroupVersionResource]*watcher{}
	for kind, gvr := range gvrs {
		// if we already have one, transfer it to the new map
		if c.dynamicWatchers[gvr] != nil {
			dynamicWatchers[gvr] = c.dynamicWatchers[gvr]
			delete(c.dynamicWatchers, gvr)
		} else {
			logger.Info("start watcher ...", "gvr", gvr)
			watchInterface, _ := c.client.GetDynamicInterface().Resource(gvr).Watch(context.TODO(), metav1.ListOptions{})
			w := &watcher{
				watcher: watchInterface,
				gvk:     gvr.GroupVersion().WithKind(kind),
				hashes:  map[types.UID]Resource{},
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
			objs, _ := c.client.GetDynamicInterface().Resource(gvr).List(context.TODO(), metav1.ListOptions{})
			for _, obj := range objs.Items {
				uid := obj.GetUID()
				hash := reportutils.CalculateResourceHash(obj)
				w.hashes[uid] = Resource{
					Hash:      hash,
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
				}
				c.notify(uid, w.gvk, w.hashes[uid])
			}
		}
	}
	// shutdown remaining watcher
	for gvr, watcher := range c.dynamicWatchers {
		watcher.watcher.Stop()
		delete(c.dynamicWatchers, gvr)
	}
	c.dynamicWatchers = dynamicWatchers
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

func (c *controller) reconcile(logger logr.Logger, key, namespace, name string) error {
	return c.updateDynamicWatchers()
}
