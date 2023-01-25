package policycache

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	pcache "github.com/kyverno/kyverno/pkg/policycache"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 3
	ControllerName = "policycache-controller"
	maxRetries     = 10
)

type Controller interface {
	controllers.Controller
	WarmUp() error
}

type controller struct {
	cache pcache.Cache

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	// queue
	queue workqueue.RateLimitingInterface

	// client
	client dclient.Interface
}

func NewController(client dclient.Interface, pcache pcache.Cache, cpolInformer kyvernov1informers.ClusterPolicyInformer, polInformer kyvernov1informers.PolicyInformer) Controller {
	c := controller{
		cache:      pcache,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		client:     client,
	}
	_, _ = cpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addPolicyCache,
		UpdateFunc: c.updatePolicyCache,
		DeleteFunc: c.deletePolicyCache,
	})

	_, _ = polInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addPolicyCache,
		UpdateFunc: c.updatePolicyCache,
		DeleteFunc: c.deletePolicyCache,
	})
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	return &c
}

func (c *controller) WarmUp() error {
	logger.Info("warming up ...")
	defer logger.Info("warm up done")

	pols, err := c.polLister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, policy := range pols {
		if key, err := cache.MetaNamespaceKeyFunc(policy); err != nil {
			return err
		} else {
			subresourceGVKToKind := getSubresourceGVKToKindMap(policy, c.client)
			c.cache.Set(key, policy, subresourceGVKToKind)
		}
	}
	cpols, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, policy := range cpols {
		if key, err := cache.MetaNamespaceKeyFunc(policy); err != nil {
			return err
		} else {
			subresourceGVKToKind := getSubresourceGVKToKindMap(policy, c.client)
			c.cache.Set(key, policy, subresourceGVKToKind)
		}
	}
	return nil
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.loadPolicy(namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.cache.Unset(key)
		}
		return err
	}
	// TODO: check resource version ?
	subresourceGVKToKind := getSubresourceGVKToKindMap(policy, c.client)
	c.cache.Set(key, policy, subresourceGVKToKind)
	return nil
}

func (c *controller) loadPolicy(namespace, name string) (kyvernov1.PolicyInterface, error) {
	if namespace == "" {
		return c.cpolLister.Get(name)
	} else {
		return c.polLister.Policies(namespace).Get(name)
	}
}

func getSubresourceGVKToKindMap(policy kyvernov1.PolicyInterface, client dclient.Interface) map[string]string {
	subresourceGVKToKind := make(map[string]string)
	for _, rule := range autogen.ComputeRules(policy) {
		for _, gvk := range rule.MatchResources.GetKinds() {
			gv, k := kubeutils.GetKindFromGVK(gvk)
			_, subresource := kubeutils.SplitSubresource(k)
			if subresource != "" {
				apiResource, _, _, _ := client.Discovery().FindResource(gv, k)
				subresourceGVKToKind[gvk] = apiResource.Kind
			}
		}
	}
	return subresourceGVKToKind
}

func (pc *controller) enqueuePolicyCache(policy kyvernov1.PolicyInterface) {
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	pc.queue.Add(key)
}

func (c *controller) addPolicyCache(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)

	logger.Info("policy created", "uid", p.UID, "kind", "ClusterPolicy", "name", p.Name)

	logger.V(4).Info("queuing policy for background processing", "name", p.Name)
	c.enqueuePolicyCache(p)
}

func (c *controller) updatePolicyCache(old, cur interface{}) {
	oldP := old.(*kyvernov1.ClusterPolicy)
	curP := cur.(*kyvernov1.ClusterPolicy)

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	logger.V(2).Info("updating policy", "name", oldP.Name)

	c.enqueuePolicyCache(curP)
}

func (c *controller) deletePolicyCache(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.Info("policy deleted", "uid", p.UID, "kind", "ClusterPolicy", "name", p.Name)

	rules := autogen.ComputeRules(p)
	for _, r := range rules {
		clone, sync := r.GetCloneSyncForGenerate()
		if clone && sync {
			return
		}
	}
	c.enqueuePolicyCache(p)
}
