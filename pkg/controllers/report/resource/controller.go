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
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 5
	workers    = 1
)

type Resource struct {
	Namespace string
	Name      string
	Gvk       schema.GroupVersionKind
	Hash      string
}

type EventHandler func(types.UID, Resource)

type MetadataCache interface {
	GetResourceHash(uid types.UID) (Resource, bool)
	AddEventHandler(EventHandler)
}

type Controller interface {
	controllers.Controller
	MetadataCache
}

type informer struct {
	watcher watch.Interface
	gvk     schema.GroupVersionKind
}

type controller struct {
	// clients
	client         dclient.Interface
	metadataClient metadata.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister

	// queue
	queue workqueue.RateLimitingInterface

	lock              sync.Mutex
	metadataInformers map[schema.GroupVersionResource]*informer
	hashes            map[types.UID]Resource
	eventHandlers     []EventHandler
}

func NewController(
	client dclient.Interface,
	metadataClient metadata.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
) Controller {
	c := controller{
		client:            client,
		metadataClient:    metadataClient,
		polLister:         polInformer.Lister(),
		cpolLister:        cpolInformer.Lister(),
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		metadataInformers: map[schema.GroupVersionResource]*informer{},
		hashes:            map[types.UID]Resource{},
	}
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) GetResourceHash(uid types.UID) (Resource, bool) {
	ret, exists := c.hashes[uid]
	return ret, exists
}

func (c *controller) AddEventHandler(eventHandler EventHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.eventHandlers = append(c.eventHandlers, eventHandler)
	for uid, resource := range c.hashes {
		eventHandler(uid, resource)
	}
}

func (c *controller) updateMetadataInformers() error {
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
	metadataInformers := map[schema.GroupVersionResource]*informer{}
	for kind, gvr := range gvrs {
		// if we already have one, transfer it to the new map
		if c.metadataInformers[gvr] != nil {
			metadataInformers[gvr] = c.metadataInformers[gvr]
			delete(c.metadataInformers, gvr)
		} else {
			logger.Info("start metadata informer ...", "gvr", gvr)
			watcher, _ := c.client.GetDynamicInterface().Resource(gvr).Watch(context.TODO(), metav1.ListOptions{})
			i := &informer{
				watcher: watcher,
				gvk:     gvr.GroupVersion().WithKind(kind),
			}
			go func() {
				for event := range watcher.ResultChan() {
					switch event.Type {
					case watch.Added:
						c.updateHash(event.Object.(*unstructured.Unstructured), i.gvk)
					case watch.Modified:
						c.updateHash(event.Object.(*unstructured.Unstructured), i.gvk)
					case watch.Deleted:
						c.deleteHash(event.Object.(*unstructured.Unstructured))
					}
				}
				logger.Info("stopping watch routine")
			}()
			metadataInformers[gvr] = i
			objs, _ := c.client.GetDynamicInterface().Resource(gvr).List(context.TODO(), metav1.ListOptions{})
			for _, obj := range objs.Items {
				uid := obj.GetUID()
				hash := reportutils.CalculateResourceHash(obj)
				c.hashes[uid] = Resource{
					Hash:      hash,
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
					Gvk:       i.gvk,
				}
			}
		}
	}
	// shutdown remaining informers
	for gvr, informer := range c.metadataInformers {
		logger.Info("stop metadata informer ...", "gvr", gvr)
		informer.watcher.Stop()
		delete(c.metadataInformers, gvr)
	}
	c.metadataInformers = metadataInformers
	return nil
}

func (c *controller) notify(uid types.UID, obj Resource) {
	for _, handler := range c.eventHandlers {
		handler(uid, obj)
	}
}

func (c *controller) updateHash(obj *unstructured.Unstructured, gvk schema.GroupVersionKind) {
	c.lock.Lock()
	defer c.lock.Unlock()
	uid := obj.GetUID()
	hash := reportutils.CalculateResourceHash(*obj)
	c.hashes[uid] = Resource{
		Hash:      hash,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Gvk:       gvk,
	}
	c.notify(uid, c.hashes[uid])
}

func (c *controller) deleteHash(obj *unstructured.Unstructured) {
	c.lock.Lock()
	defer c.lock.Unlock()
	uid := obj.GetUID()
	hash := c.hashes[uid]
	delete(c.hashes, uid)
	c.notify(uid, hash)
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

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.V(3).Info("reconciling ...")
	return c.updateMetadataInformers()
}
