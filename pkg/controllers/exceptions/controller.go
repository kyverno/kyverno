package exceptions

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
)

type ruleIndex = map[string][]*kyvernov2alpha1.PolicyException

type policyIndex = map[string]ruleIndex

type controller struct {
	// listers
	cpolLister  kyvernov1listers.ClusterPolicyLister
	polLister   kyvernov1listers.PolicyLister
	polexLister kyvernov2alpha1listers.PolicyExceptionLister

	// queue
	queue workqueue.RateLimitingInterface

	// state
	lock      sync.RWMutex
	index     policyIndex
	namespace string
}

const (
	maxRetries     = 10
	Workers        = 3
	ControllerName = "exceptions-controller"
)

func NewController(
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	polexInformer kyvernov2alpha1informers.PolicyExceptionInformer,
	namespace string,
) *controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), queue)

	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), queue)

	c := &controller{
		cpolLister:  cpolInformer.Lister(),
		polLister:   polInformer.Lister(),
		polexLister: polexInformer.Lister(),
		queue:       queue,
		index:       policyIndex{},
		namespace:   namespace,
	}
	controllerutils.AddEventHandlersT(polexInformer.Informer(), c.addPolex, c.updatePolex, c.deletePolex)
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) Find(policyName string, ruleName string) ([]*kyvernov2alpha1.PolicyException, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.index[policyName][ruleName], nil
}

func (c *controller) addPolex(polex *kyvernov2alpha1.PolicyException) {
	names := sets.New[string]()
	for _, ex := range polex.Spec.Exceptions {
		names.Insert(ex.PolicyName)
	}
	for name := range names {
		c.queue.Add(name)
	}
}

func (c *controller) updatePolex(old *kyvernov2alpha1.PolicyException, new *kyvernov2alpha1.PolicyException) {
	names := sets.New[string]()
	for _, ex := range old.Spec.Exceptions {
		names.Insert(ex.PolicyName)
	}
	for _, ex := range new.Spec.Exceptions {
		names.Insert(ex.PolicyName)
	}
	for name := range names {
		c.queue.Add(name)
	}
}

func (c *controller) deletePolex(polex *kyvernov2alpha1.PolicyException) {
	names := sets.New[string]()
	for _, ex := range polex.Spec.Exceptions {
		names.Insert(ex.PolicyName)
	}
	for name := range names {
		c.queue.Add(name)
	}
}

func (c *controller) getPolicy(namespace, name string) (kyvernov1.PolicyInterface, error) {
	if namespace == "" {
		cpolicy, err := c.cpolLister.Get(name)
		if err != nil {
			return nil, err
		}
		return cpolicy, nil
	} else {
		policy, err := c.polLister.Policies(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return policy, nil
	}
}

func (c *controller) listExceptions() ([]*kyvernov2alpha1.PolicyException, error) {
	if c.namespace == "" {
		return c.polexLister.List(labels.Everything())
	}
	return c.polexLister.PolicyExceptions(c.namespace).List(labels.Everything())
}

func (c *controller) buildRuleIndex(key string, policy kyvernov1.PolicyInterface) (ruleIndex, error) {
	polexList, err := c.listExceptions()
	if err != nil {
		return nil, err
	}
	slices.SortFunc(polexList, func(a, b *kyvernov2alpha1.PolicyException) int {
		if cmp := cmp.Compare(a.Namespace, b.Namespace); cmp != 0 {
			return cmp
		}
		if cmp := cmp.Compare(a.Name, b.Name); cmp != 0 {
			return cmp
		}
		return 0
	})
	index := ruleIndex{}
	for _, rule := range autogen.ComputeRules(policy) {
		for _, polex := range polexList {
			if polex.Contains(key, rule.Name) {
				index[rule.Name] = append(index[rule.Name], polex)
			}
		}
	}
	return index, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.getPolicy(namespace, name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "unable to get the policy from policy informer")
			return err
		}
		c.lock.Lock()
		defer c.lock.Unlock()
		delete(c.index, key)
		return nil
	}
	ruleIndex, err := c.buildRuleIndex(key, policy)
	if err != nil {
		return err
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.index[key] = ruleIndex
	return nil
}
