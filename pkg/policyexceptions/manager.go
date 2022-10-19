package policyexceptions

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernov2beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

type PolicyExceptionManager struct {
	peInformer      kyvernov2beta1informers.PolicyExceptionInformer
	informersSynced []cache.InformerSynced
	log             logr.Logger
	namespace       string
}

func NewPolicyExceptionManager(peInformer kyvernov2beta1informers.PolicyExceptionInformer, logger logr.Logger, namespace string) *PolicyExceptionManager {
	c := &PolicyExceptionManager{
		peInformer: peInformer,
		log:        logger,
		namespace:  namespace,
	}

	c.informersSynced = []cache.InformerSynced{c.peInformer.Informer().HasSynced}

	return c
}

func (c *PolicyExceptionManager) Run(ctx context.Context, workers int) {
	logger := c.log

	defer utilruntime.HandleCrash()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("PolicyExceptionManager", ctx.Done(), c.informersSynced...) {
		return
	}

	c.peInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addPolicyException,
		UpdateFunc: c.updatePolicyException,
		DeleteFunc: c.deletePolicyException,
	})

	<-ctx.Done()
}

func (c *PolicyExceptionManager) addPolicyException(obj interface{}) {
	p := obj.(*kyvernov2beta1.PolicyException)

	logger := c.log
	logger.Info("policy exception created", "uid", p.UID, "kind", "PolicyException", "name", p.Name)
}

func (c *PolicyExceptionManager) updatePolicyException(old, cur interface{}) {
	logger := c.log

	oldP := old.(*kyvernov2beta1.PolicyException)
	curP := cur.(*kyvernov2beta1.PolicyException)

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	logger.V(4).Info("updating policy exception", "name", oldP.Name)

	// TODO: allow update by:
	// - RBAC
	// - Kyverno Policy
	// - Signed yamls
}

func (c *PolicyExceptionManager) deletePolicyException(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov2beta1.PolicyException)
	if !ok {
		c.log.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger := c.log

	logger.Info("policy exception deleted", "uid", p.UID, "kind", "PolicyException", "name", p.Name)
}

type ExceptionLister interface {
	List(selector labels.Selector) (ret []*kyvernov2beta1.PolicyException, err error)
}

func (c *PolicyExceptionManager) ExceptionsByRule(policy kyvernov1.PolicyInterface, ruleName string) ExcludeResource {
	excludeResource := ExcludeResource{}

	var lister ExceptionLister = c.peInformer.Lister()
	if c.namespace != "" {
		lister = c.peInformer.Lister().PolicyExceptions(c.namespace)
	}

	exceps, err := lister.List(labels.Everything())
	if err != nil {
		return excludeResource
	}

	for _, v := range exceps {
		exceptions := v.Spec.Exceptions

		for _, p := range exceptions {
			if p.PolicyName != policy.GetName() {
				continue
			}

			for _, r := range p.RuleNames {
				if r == ruleName {
					excludeResource = append(excludeResource, *v.Spec.Exclude.DeepCopy())
				}
			}
		}
	}

	return excludeResource
}

// isNil checks if PolicyExceptionManager is an empty manager
func (c *PolicyExceptionManager) IsNil() bool {
	return reflect.DeepEqual(*c, PolicyExceptionManager{})
}
