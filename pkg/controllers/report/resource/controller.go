package resource

import (
	"fmt"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 5
	workers    = 1
)

type EventHandler func(string, types.UID)

type MetadataCache interface {
	GetResource(types.UID) (metav1.Object, schema.GroupVersionKind, error)
	AddEventHandler(EventHandler)
}

type Controller interface {
	controllers.Controller
	MetadataCache
}

type informer struct {
	informer informers.GenericInformer
	gvk      schema.GroupVersionKind
	stop     chan struct{}
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
	eventHandlers     []EventHandler
}

func NewController(
	client dclient.Interface,
	metadataClient metadata.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
) *controller {
	c := controller{
		client:            client,
		metadataClient:    metadataClient,
		polLister:         polInformer.Lister(),
		cpolLister:        cpolInformer.Lister(),
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		metadataInformers: map[schema.GroupVersionResource]*informer{},
	}
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) GetResource(uid types.UID) (metav1.Object, schema.GroupVersionKind, error) {
	for _, i := range c.metadataInformers {
		objs, err := i.informer.Informer().GetIndexer().ByIndex("uid", string(uid))
		if err == nil && len(objs) == 1 {
			return objs[0].(metav1.Object), i.gvk, nil
		} else if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, schema.GroupVersionKind{}, err
			} else {
				logger.Error(err, "failed to query indexer")
			}
		}
	}
	return nil, schema.GroupVersionKind{}, nil
}

func (c *controller) AddEventHandler(eventHandler EventHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.eventHandlers = append(c.eventHandlers, eventHandler)
	for _, i := range c.metadataInformers {
		objs, _ := i.informer.Lister().List(labels.Everything())
		for _, obj := range objs {
			resource := obj.(metav1.Object)
			eventHandler(resource.GetNamespace(), resource.GetUID())
		}
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
			i := &informer{
				gvk: gvr.GroupVersion().WithKind(kind),
				informer: metadatainformers.NewFilteredMetadataInformer(
					c.metadataClient,
					gvr,
					"",
					time.Minute*10,
					cache.Indexers{
						"uid": func(obj interface{}) ([]string, error) {
							meta, err := meta.Accessor(obj)
							if err != nil {
								return []string{""}, fmt.Errorf("object has no meta: %v", err)
							}
							return []string{string(meta.GetUID())}, nil
						},
					},
					nil,
				),
				stop: make(chan struct{}),
			}
			i.informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc:    c.addResource,
				UpdateFunc: c.updateResource,
				DeleteFunc: c.deleteResource,
			})
			go i.informer.Informer().Run(i.stop)
			metadataInformers[gvr] = i
			cache.WaitForCacheSync(i.stop, i.informer.Informer().HasSynced)
		}
	}
	// shutdown remaining informers
	for gvr, informer := range c.metadataInformers {
		logger.Info("stop metadata informer ...", "gvr", gvr)
		close(informer.stop)
		delete(c.metadataInformers, gvr)
	}
	c.metadataInformers = metadataInformers
	return nil
}

func (c *controller) notify(obj metav1.Object) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, handler := range c.eventHandlers {
		handler(obj.GetNamespace(), obj.GetUID())
	}
}

func (c *controller) addResource(obj interface{}) {
	c.notify(obj.(metav1.Object))
}

func (c *controller) updateResource(_, obj interface{}) {
	c.notify(obj.(metav1.Object))
}

func (c *controller) deleteResource(obj interface{}) {
	c.notify(obj.(metav1.Object))
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
