package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	kyvernocontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	client dclient.Interface
	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister // not sure that we need listers for this one
	polLister  kyvernov1listers.PolicyLister

	// queue
	queue workqueue.RateLimitingInterface
}

const (
	MaxRetries = 10
	Workers    = 3
)

func NewController(dclient dclient.Interface, cpolInformer kyvernov1informers.ClusterPolicyInformer, polInformer kyvernov1informers.PolicyInformer) *controller {
	c := &controller{
		client:     dclient,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
	}

	cpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj)
		},
	})
	polInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj)
		},
	})
	return c
}

func (c *controller) enqueue(obj interface{}) {
	c.queue.Add(obj)
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, Workers, MaxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	var resp *response.EngineResponse
	var policyToCheck kyvernov1.PolicyInterface
	contx := kyvernocontext.NewContext()
	policy, err := c.getPolicy(namespace, name)
	if err != nil {
		logger.Error(err, "unable to get the policy from policy informer")
		return err
	}

	for _, rule := range policy.GetSpec().Rules {
		if !rule.HasCleanUp() {
			continue
		}
		triggers := generateTriggers(c.client, rule, logger)
		policyToCheck = getPolicyToCheck(rule, namespace)
		for _, trigger := range triggers {
			policyCtx := &engine.PolicyContext{
				Policy:      policyToCheck,
				NewResource: *trigger,
				JSONContext: contx,
				Client:      c.client,
			}
			resp = engine.Cleanup(policyCtx)
			if len(resp.PolicyResponse.Rules) == 0 {
				continue
			}
			if resp.PolicyResponse.Rules[0].Status == response.RuleStatusPass {
				cronjob := getCronJobForTriggerResource(rule, trigger)
				_, err = c.client.CreateResource("batch/v1", "CronJob", trigger.GetNamespace(), cronjob, false)
				if err != nil {
					logger.Error(err, "unable to create the resource of kind CronJob for cleanup rule in policy %s", name)
					return err
				}
			}

		}
	}

	return nil
}
