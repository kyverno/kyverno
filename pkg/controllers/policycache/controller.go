package policycache

import (
	"encoding/json"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	pcache "github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	cache pcache.Cache

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	cpolV2beta1Lister kyvernov2listers.ClusterPolicyLister
	polV2beta1Lister  kyvernov2listers.PolicyLister

	// cpolSynced returns true if the cluster policy shared informer has synced at least once
	cpolSynced cache.InformerSynced
	// polSynced returns true if the policy shared informer has synced at least once
	polSynced cache.InformerSynced

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(pcache pcache.Cache, cpolV1Informer kyvernov1informers.ClusterPolicyInformer, polv1Informer kyvernov1informers.PolicyInformer, cpolV2Informer kyvernov2informers.ClusterPolicyInformer, polv2nformer kyvernov2informers.PolicyInformer) *controller {
	c := controller{
		cache:             pcache,
		cpolLister:        cpolV1Informer.Lister(),
		polLister:         polv1Informer.Lister(),
		cpolV2beta1Lister: cpolV2Informer.Lister(),
		polV2beta1Lister:  polv2nformer.Lister(),
		cpolSynced:        cpolV2Informer.Informer().HasSynced,
		polSynced:         polv2nformer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policycache-controller"),
	}
	controllerutils.AddDefaultEventHandlers(logger, cpolV1Informer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polv1Informer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolV2Informer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polv2nformer.Informer(), c.queue)
	return &c
}

func (c *controller) WarmUp() error {
	logger.Info("warming up ...")
	defer logger.Info("warm up done")

	pols, err := c.polV2beta1Lister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, policy := range pols {
		if key, err := cache.MetaNamespaceKeyFunc(policy); err != nil {
			return err
		} else {
			if utils.IsConversionRequired(policy.GetSpec()) {
				v1Policy, _ := c.polLister.Policies(policy.Namespace).Get(policy.Name)
				policyBytes := utils.ConvertPolicyToV2(v1Policy, nil)
				var policy *kyvernov2.Policy
				if err := json.Unmarshal(policyBytes, &policy); err != nil {
					return err
				}
				c.cache.Set(key, policy)
			} else {
				c.cache.Set(key, policy)
			}

		}
	}
	cpols, err := c.polV2beta1Lister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, policy := range cpols {
		if key, err := cache.MetaNamespaceKeyFunc(policy); err != nil {
			return err
		} else {
			if utils.IsConversionRequired(policy.GetSpec()) {
				v1ClusterPolicy, _ := c.cpolLister.Get(policy.Name)
				policyBytes := utils.ConvertPolicyToV2(nil, v1ClusterPolicy)
				var cpolicy *kyvernov2.ClusterPolicy
				if err := json.Unmarshal(policyBytes, &cpolicy); err != nil {
					return err
				}
				c.cache.Set(key, cpolicy)

			} else {
				c.cache.Set(key, policy)
			}
		}
	}
	return nil
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run("policycache-controller", logger, c.queue, workers, maxRetries, c.reconcile, stopCh, c.cpolSynced, c.polSynced)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger.Info("reconciling ...", "key", key, "namespace", namespace, "name", name)
	policy, err := c.loadPolicy(namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.cache.Unset(key)
		}
		return err
	}
	// TODO: check resource version ?
	c.cache.Set(key, policy)
	return nil
}

func (c *controller) loadPolicy(namespace, name string) (kyvernov2.PolicyInterface, error) {
	if namespace == "" {
		return c.cpolV2beta1Lister.Get(name)
	} else {
		return c.polV2beta1Lister.Policies(namespace).Get(name)
	}
}
