package webhookconfig

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TODO:
// 1. configure timeout
// 2.wildcard support
type webhookConfigManager struct {
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset

	pInformer  kyvernoinformer.ClusterPolicyInformer
	npInformer kyvernoinformer.PolicyInformer

	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister

	// pListerSynced returns true if the cluster policy store has been synced at least once
	pListerSynced cache.InformerSynced

	// npListerSynced returns true if the namespace policy store has been synced at least once
	npListerSynced cache.InformerSynced

	resCache resourcecache.ResourceCache

	queue workqueue.RateLimitingInterface

	// matchAllKinds indicates whether the existing policies has * defined for the matching kind
	matchAllKinds bool

	stopCh <-chan struct{}

	log logr.Logger
}

type manage interface {
	start()
	sync(key string) error
}

func newWebhookConfigManager(
	client *client.Client,
	kyvernoClient *kyvernoclient.Clientset,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	resCache resourcecache.ResourceCache,
	stopCh <-chan struct{},
	log logr.Logger) manage {

	m := &webhookConfigManager{
		client:        client,
		kyvernoClient: kyvernoClient,
		pInformer:     pInformer,
		npInformer:    npInformer,
		resCache:      resCache,
		stopCh:        stopCh,
		log:           log,
	}

	m.pLister = pInformer.Lister()
	m.npLister = npInformer.Lister()

	m.pListerSynced = pInformer.Informer().HasSynced
	m.npListerSynced = npInformer.Informer().HasSynced

	return m
}

// start is a blocking call to configure webhook
func (m *webhookConfigManager) start() {
	defer utilruntime.HandleCrash()
	defer m.queue.ShutDown()

	m.log.Info("starting")
	defer m.log.Info("shutting down")

	if !cache.WaitForCacheSync(m.stopCh, m.pListerSynced, m.npListerSynced) {
		m.log.Info("failed to sync informer cache")
		return
	}

	m.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.handleClusterPolicy,
		UpdateFunc: m.updatePolicy,
		DeleteFunc: m.handleClusterPolicy,
	})

	m.npInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.handlePolicy,
		UpdateFunc: m.updateNsPolicy,
		DeleteFunc: m.handlePolicy,
	})

	for m.processNextWorkItem() {
	}
}

func (m *webhookConfigManager) processNextWorkItem() bool {
	key, quit := m.queue.Get()
	if quit {
		return false
	}
	defer m.queue.Done(key)
	err := m.sync(key.(string))
	m.handleErr(err, key)

	return true
}

func (m *webhookConfigManager) sync(key string) error {
	logger := m.log.WithName("sync")
	startTime := time.Now()
	logger.V(4).Info("started syncing policy", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing policy", "key", key, "processingTime", time.Since(startTime).String())
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Info("invalid resource key", "key", key)
		return nil
	}

	policy, err := m.getPolicy(namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return m.reconcileWebhook(policy)
}

// - list current policies
// - build webhook object
// - fetch and compare with fetched webhook configuration, update if not the same
// - update current policy status
func (m *webhookConfigManager) reconcileWebhook(policy *kyverno.ClusterPolicy) error {
	logger := m.log.WithName("reconcileWebhook").WithValues("namespace", policy.GetNamespace(), "policy", policy.GetName())

	policies, err := m.listPolicies(policy.GetNamespace(), *policy.Spec.FailurePolicy)
	if err != nil {
		logger.Error(err, "cannot list current policies")
		return err
	}

	webhook := buildWebhooks(policies)
	if err = m.updateWebhookConfig(webhook); err != nil {
		logger.Error(err, "failed to update webhook configuration")
		return err
	}

	return m.updateStatus(policy)
}

func (m *webhookConfigManager) listPolicies(namespace string, failurePolicy kyverno.FailurePolicyType) ([]kyverno.ClusterPolicy, error) {
	if namespace != "" {
		polList, err := m.kyvernoClient.KyvernoV1().Policies(namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to list Policy")
		}

		policies := make([]kyverno.ClusterPolicy, len(polList.Items))
		for _, pol := range polList.Items {
			if *pol.Spec.FailurePolicy == failurePolicy {
				policies = append(policies, kyverno.ClusterPolicy(pol))
			}
		}
		return policies, nil
	}

	cpolList, err := m.kyvernoClient.KyvernoV1().ClusterPolicies().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list ClusterPolicy")
	}

	policies := make([]kyverno.ClusterPolicy, len(cpolList.Items))
	for _, cpol := range cpolList.Items {
		if *cpol.Spec.FailurePolicy == failurePolicy {
			policies = append(policies, cpol)
		}
	}
	return policies, nil
}

func (m *webhookConfigManager) updateWebhookConfig(webhook map[string]interface{}) error {
	return nil
}

func (m *webhookConfigManager) updateStatus(policy *kyverno.ClusterPolicy) error {
	policyCopy := policy.DeepCopy()
	policyCopy.Status.Ready = true
	if policy.GetNamespace() == "" {
		_, err := m.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), policyCopy, v1.UpdateOptions{})
		return err
	}

	_, err := m.kyvernoClient.KyvernoV1().Policies(policyCopy.GetNamespace()).UpdateStatus(context.TODO(), (*kyverno.Policy)(policyCopy), v1.UpdateOptions{})
	return err
}

func (m *webhookConfigManager) getPolicy(namespace, name string) (*kyverno.ClusterPolicy, error) {
	// TODO: test default/policy
	if namespace == "" {
		return m.pLister.Get(name)
	}

	nsPolicy, err := m.npLister.Policies(namespace).Get(name)
	if err == nil && nsPolicy != nil {
		p := kyverno.ClusterPolicy(*nsPolicy)
		return &p, err
	}

	return nil, err
}
func (m *webhookConfigManager) handleErr(err error, key interface{}) {
	logger := m.log
	if err == nil {
		m.queue.Forget(key)
		return
	}

	if m.queue.NumRequeues(key) < 3 {
		logger.Error(err, "failed to sync policy", "key", key)
		m.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	logger.V(2).Info("dropping policy out of queue", "key", key)
	m.queue.Forget(key)
}

func (m *webhookConfigManager) handleClusterPolicy(obj interface{}) {
	p, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		p, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		m.log.V(4).Info("Recovered deleted ClusterPolicy '%s' from tombstone", "name", p.GetName())
	}

	m.enqueue(p)
}

func (m *webhookConfigManager) updatePolicy(old, cur interface{}) {
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	m.enqueue(curP)
}

func (m *webhookConfigManager) handlePolicy(obj interface{}) {
	p, ok := obj.(*kyverno.Policy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		p, ok = tombstone.Obj.(*kyverno.Policy)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		m.log.V(4).Info("Recovered deleted Policy '%s' from tombstone", "name", p.GetName())
	}

	pol := kyverno.ClusterPolicy(*p)
	m.enqueue(&pol)
}

func (m *webhookConfigManager) updateNsPolicy(old, cur interface{}) {
	oldP := old.(*kyverno.Policy)
	curP := cur.(*kyverno.Policy)

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	pol := kyverno.ClusterPolicy(*curP)
	m.enqueue(&pol)
}

func (m *webhookConfigManager) enqueue(policy *kyverno.ClusterPolicy) {
	logger := m.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	m.queue.Add(key)
}

func buildWebhooks(policies []kyverno.ClusterPolicy) map[string]interface{} {
	return nil
}
