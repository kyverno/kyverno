package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernovv1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformerv1 "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernoinformerv1beta1 "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolisterv1beta1 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = log.Log.WithName("cleanup-controller")

type Controller2 struct {
	client        *dclient.Client
	kyvernoClient *kyvernoclient.Clientset
	pInformer     kyvernoinformerv1.ClusterPolicyInformer
	grInformer    kyvernoinformerv1.GenerateRequestInformer
	urLister      kyvernolisterv1beta1.UpdateRequestNamespaceLister
	queue         workqueue.RateLimitingInterface
}

//NewController returns a new controller instance to manage generate-requests
func NewController2(
	kubeClient kubernetes.Interface,
	kyvernoclient *kyvernoclient.Clientset,
	client *dclient.Client,
	pInformer kyvernoinformerv1.ClusterPolicyInformer,
	npInformer kyvernoinformerv1.PolicyInformer,
	grInformer kyvernoinformerv1.GenerateRequestInformer,
	urInformer kyvernoinformerv1beta1.UpdateRequestInformer,
	namespaceInformer coreinformers.NamespaceInformer,
	log logr.Logger,
) (*Controller2, error) {
	c := Controller2{
		kyvernoClient: kyvernoclient,
		client:        client,
		pInformer:     pInformer,
		grInformer:    grInformer,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate-request-cleanup"),
	}
	c.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addClusterPolicy,
		UpdateFunc: c.updateClusterPolicy,
		DeleteFunc: c.deleteClusterPolicy,
	})
	return &c, nil
}

func (c *Controller2) addClusterPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.enqueueClusterPolicy(p)
}

func (c *Controller2) updateClusterPolicy(old, cur interface{}) {
	p := cur.(*kyvernov1.ClusterPolicy)
	c.enqueueClusterPolicy(p)
}

func (c *Controller2) deleteClusterPolicy(obj interface{}) {
	p, ok := obj.(*kyvernov1.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		p, ok = tombstone.Obj.(*kyvernov1.ClusterPolicy)
		if !ok {
			return
		}
	}
	c.enqueueClusterPolicy(p)
}

//Run starts the generate-request re-conciliation loop
func (c *Controller2) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	// if !cache.WaitForCacheSync(stopCh, c.pSynced, c.grSynced, c.urSynced, c.npSynced, c.nsListerSynced) {
	// 	return
	// }

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller2) enqueueClusterPolicy(obj *kyvernov1.ClusterPolicy) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.queue.Add(key)
}

func (c *Controller2) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller2) processNextWorkItem() bool {
	if key, quit := c.queue.Get(); !quit {
		defer c.queue.Done(key)
		c.handleErr(c.reconcileClusterPolicy(key.(string)), key)
		return true
	}
	return false
}

func (c *Controller2) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}
	if apierrors.IsNotFound(err) {
		c.queue.Forget(key)
		return
	}
	if c.queue.NumRequeues(key) < maxRetries {
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
}

const cleanupFinalizer = "cleanup.kyverno.io/finalizer"

func (c *Controller2) reconcileClusterPolicy(key string) error {
	_, n, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	p, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().Get(context.TODO(), n, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if p.DeletionTimestamp.IsZero() {
		if !kubeutils.ContainsFinalizer(p, cleanupFinalizer) {
			kubeutils.AddFinalizer(p, cleanupFinalizer)
			_, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().Update(context.TODO(), p, metav1.UpdateOptions{})
			return err
		}
	} else {
		if kubeutils.ContainsFinalizer(p, cleanupFinalizer) {
			if err := c.cleanupClusterPolicy(p); err != nil {
				return err
			}
			kubeutils.RemoveFinalizer(p, cleanupFinalizer)
			_, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().Update(context.TODO(), p, metav1.UpdateOptions{})
			return err
		}
		return nil
	}
	return nil
}

func (c *Controller2) cleanupClusterPolicy(p *kyvernov1.ClusterPolicy) error {
	rules := autogen.ComputeRules(p)
	generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(rules, c.client, p.GetName(), logger)
	if !generatePolicyWithClone {
		urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(p.Name)
		if err != nil {
			return err
		}
		for _, ur := range urs {
			for _, rule := range rules {
				if err := c.cleanupGeneratedResourcesForRule(ur, rule); err != nil {
					return err
				}
			}
			if err := c.cleanupUpdateRequest(ur); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Controller2) cleanupGeneratedResourcesForRule(ur *kyvernovv1beta1.UpdateRequest, rule kyvernov1.Rule) error {
	// TODO
	// - find resources generated by the rule
	// - delete the resources
	return nil
}

func (c *Controller2) cleanupUpdateRequest(ur *kyvernovv1beta1.UpdateRequest) error {
	return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
}
