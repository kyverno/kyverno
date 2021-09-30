package webhookconfig

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var defaultWebhookTimeout int64 = 3

// TODO:
// 1. configure timeout
// 2.wildcard support
// webhookConfigManager manges the webhook configuration dynamically
// it is NOT multi-thread safe
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
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmanager"),
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

func (m *webhookConfigManager) reconcileWebhook(policy *kyverno.ClusterPolicy) error {
	logger := m.log.WithName("reconcileWebhook").WithValues("namespace", policy.GetNamespace(), "policy", policy.GetName())

	policies, err := m.listPolicies(policy.GetNamespace(), *policy.Spec.FailurePolicy)
	if err != nil {
		logger.Error(err, "cannot list current policies")
		return err
	}

	webhooks := m.buildWebhooks(policies)
	if err = m.updateWebhookConfig(webhooks); err != nil {
		return errors.Wrapf(err, "failed to update webhook configurations for policy %s/$s", policy.GetNamespace(), policy.GetName())
	}

	if err := m.updateStatus(policy); err != nil {
		return errors.Wrapf(err, "failed to update policy status %s/$s", policy.GetNamespace(), policy.GetName())
	}

	logger.Info("policy %s/%s is ready to serve admission requests", policy.GetNamespace(), policy.GetName())
	return nil
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

const (
	apiGroups   string = "apiGroups"
	apiVersions string = "apiVersions"
	resources   string = "resources"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on kind, failurePolicy and webhookTimeout
type webhook struct {
	kind              string
	maxWebhookTimeout int64
	failurePolicy     kyverno.FailurePolicyType

	// rule represents the same rule struct of the webhook using a map object
	// https://github.com/kubernetes/api/blob/master/admissionregistration/v1/types.go#L25
	rule map[string]interface{}
}

func (m *webhookConfigManager) updateWebhookConfig(webhooks []*webhook) error {
	logger := m.log.WithName("updateWebhookConfig")
	webhooksMap := make(map[string]interface{}, len(webhooks))
	for _, w := range webhooks {
		key := strings.Join([]string{w.kind, string(w.failurePolicy)}, "/")
		webhooksMap[key] = w
	}

	var errs []string
	if err := m.compareAndUpdateWebhook(kindMutating, getResourceMutatingWebhookConfigName(""), webhooksMap); err != nil {
		logger.V(4).Info("failed to update mutatingwebhookconfigurations", "error", err.Error())
		errs = append(errs, err.Error())
	}

	if err := m.compareAndUpdateWebhook(kindValidating, getResourceValidatingWebhookConfigName(""), webhooksMap); err != nil {
		logger.V(4).Info("failed to update validatingwebhookconfigurations", "error", err.Error())
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (m *webhookConfigManager) compareAndUpdateWebhook(webhookKind, webhookName string, webhooksMap map[string]interface{}) error {
	logger := m.log.WithName("compareAndUpdateWebhook").WithValues("kind", webhookKind, "name", webhookName)
	webhookCache, _ := m.resCache.GetGVRCache(webhookKind)
	resourceWebhook, err := webhookCache.Lister().Get(webhookName)
	if err != nil {
		return errors.Wrapf(err, "unable to get %s/%s", webhookKind, webhookName)
	}

	webhooksUntyped, _, err := unstructured.NestedSlice(resourceWebhook.UnstructuredContent(), "webhooks")
	if err != nil {
		return errors.Wrapf(err, "unable to fetch tag webhooks for %s/%s", webhookKind, webhookName)
	}

	newWebooks := make([]interface{}, len(webhooksUntyped))
	copy(newWebooks, webhooksUntyped)
	var changed bool
	for i, webhookUntyed := range webhooksUntyped {
		existingWebhook, ok := webhookUntyed.(map[string]interface{})
		if !ok {
			logger.Error(errors.New("type mismatched"), "expected map[string]interface{}, got %T", webhooksUntyped)
			continue
		}

		failurePolicy, _, err := unstructured.NestedString(existingWebhook, "failurePolicy")
		if err != nil {
			logger.Error(errors.New("type mismatched"), "expected string, got %T", failurePolicy)
			continue

		}

		rules, _, err := unstructured.NestedSlice(existingWebhook, "rules")
		if err != nil {
			logger.Error(err, "type mismatched, expected []interface{}, got %T", rules)
			continue
		}

		newWebhook := webhooksMap[strings.Join([]string{webhookKind, failurePolicy}, "/")]
		w, ok := newWebhook.(*webhook)
		if !ok {
			logger.Error(errors.New("type mismatched"), "expected *webhook, got %T", newWebooks)
			continue
		}

		if !reflect.DeepEqual(w.rule, map[string]interface{}{}) && !reflect.DeepEqual(rules, []interface{}{w.rule}) {
			changed = true

			tmpRules := newWebooks[i].(map[string]interface{})["rules"].([]interface{})
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[apiGroups].([]string), apiGroups); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiGroups)
			}
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[apiVersions].([]string), apiVersions); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiVersions)
			}
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[resources].([]string), resources); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, resources)
			}

		}

		if err = unstructured.SetNestedField(newWebooks[i].(map[string]interface{}), w.maxWebhookTimeout, "timeoutSeconds"); err != nil {
			return errors.Wrapf(err, "unable to set webhooks[%d].timeoutSeconds to %v", i, w.maxWebhookTimeout)
		}
	}

	if changed {
		logger.V(4).Info("webhook configuration has been changed, updating")
		if err := unstructured.SetNestedSlice(resourceWebhook.UnstructuredContent(), newWebooks, "webhooks"); err != nil {
			return errors.Wrap(err, "unable to set new webhooks")
		}

		if _, err := m.client.UpdateResource(resourceWebhook.GetAPIVersion(), resourceWebhook.GetKind(), "", resourceWebhook, false); err != nil {
			return errors.Wrapf(err, "unable to update %s/%s: %s", resourceWebhook.GetAPIVersion(), resourceWebhook.GetKind(), resourceWebhook.GetName())
		}
		logger.V(4).Info("successfully updated the webhook configuration")
	}

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

func (m *webhookConfigManager) buildWebhooks(policies []kyverno.ClusterPolicy) (res []*webhook) {
	mutateIgnore := newWebhook(kindMutating, defaultWebhookTimeout, kyverno.Ignore)
	mutateFail := newWebhook(kindMutating, defaultWebhookTimeout, kyverno.Fail)
	validateIgnore := newWebhook(kindValidating, defaultWebhookTimeout, kyverno.Ignore)
	validateFail := newWebhook(kindValidating, defaultWebhookTimeout, kyverno.Fail)

	for _, p := range policies {
		if p.HasValidate() {
			if p.Spec.FailurePolicy != nil && *p.Spec.FailurePolicy == kyverno.Ignore {
				m.mergeWebhook(validateIgnore, p, true)
			} else {
				m.mergeWebhook(validateFail, p, true)
			}
		}

		if p.HasMutate() || p.HasGenerate() {
			if p.Spec.FailurePolicy != nil && *p.Spec.FailurePolicy == kyverno.Ignore {
				m.mergeWebhook(mutateIgnore, p, false)
			} else {
				m.mergeWebhook(mutateFail, p, false)
			}
		}
	}

	res = append(res, mutateIgnore, mutateFail, validateIgnore, validateFail)
	return res
}

// mergeWebhook merges the matching kinds of the policy to webhook.rule
func (m *webhookConfigManager) mergeWebhook(dst *webhook, policy kyverno.ClusterPolicy, isValidate bool) {
	matchedGVK := make([]string, 0)
	for _, rule := range policy.Spec.Rules {
		if isValidate && rule.HasValidate() {
			matchedGVK = append(matchedGVK, rule.MatchKinds()...)
		} else {
			matchedGVK = append(matchedGVK, rule.MatchKinds()...)
		}
	}

	gvkMap := make(map[string]int)
	gvrList := make([]schema.GroupVersionResource, 0)
	for _, gvk := range matchedGVK {
		if _, ok := gvkMap[gvk]; !ok {
			gvkMap[gvk] = 1

			// note: webhook stores GVR in its rules while policy stores GVK in its rules definition
			gv, k := common.GetKindFromGVK(gvk)
			_, gvr, err := m.client.DiscoveryClient.FindResource(gv, k)
			if err != nil {
				continue
			}
			gvrList = append(gvrList, gvr)
		}
	}

	var groups, versions, rsrcs []string
	if val, ok := dst.rule[apiGroups]; ok {
		copy(groups, val.([]string))
	}

	if val, ok := dst.rule[apiVersions]; ok {
		copy(groups, val.([]string))
	}
	if val, ok := dst.rule[resources]; ok {
		copy(groups, val.([]string))
	}

	for _, gvr := range gvrList {
		groups = append(groups, gvr.Group)
		versions = append(versions, gvr.Version)
		rsrcs = append(rsrcs, gvr.Resource)
	}

	dst.rule[apiGroups] = removeDuplicates(groups)
	dst.rule[apiVersions] = removeDuplicates(versions)
	dst.rule[resources] = removeDuplicates(rsrcs)

	if policy.Spec.WebhookTimeoutSeconds != nil {
		if dst.maxWebhookTimeout < int64(*policy.Spec.WebhookTimeoutSeconds) {
			dst.maxWebhookTimeout = int64(*policy.Spec.WebhookTimeoutSeconds)
		}
	}
}

func removeDuplicates(items []string) (res []string) {
	set := make(map[string]int)
	for _, item := range items {
		if _, ok := set[item]; !ok {
			set[item] = 1
			res = append(res, item)
		}
	}
	return
}

func newWebhook(kind string, timeout int64, failurePolicy kyverno.FailurePolicyType) *webhook {
	return &webhook{
		kind:              kind,
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rule:              make(map[string]interface{}),
	}
}
