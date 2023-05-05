package policycache

import (
	"context"
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
				apiResource, _, _, err := client.Discovery().FindResource(gv, k)
				if err != nil {
					logger.Error(err, "failed to fetch resource group versions", "gv", gv, "kind", k)
					continue
				}
				subresourceGVKToKind[gvk] = apiResource.Kind
			}
		}
	}
	return subresourceGVKToKind
}
