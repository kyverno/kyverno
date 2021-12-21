package webhookconfig

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	admregapi "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var DefaultWebhookTimeout int64 = 10

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

	mutateInformer         cache.SharedIndexInformer
	validateInformer       cache.SharedIndexInformer
	mutateInformerSynced   cache.InformerSynced
	validateInformerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	// serverIP used to get the name of debug webhooks
	serverIP string

	autoUpdateWebhooks bool

	// wildcardPolicy indicates the number of policies that matches all kinds (*) defined
	wildcardPolicy int64

	createDefaultWebhook chan<- string

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
	serverIP string,
	autoUpdateWebhooks bool,
	createDefaultWebhook chan<- string,
	stopCh <-chan struct{},
	log logr.Logger) manage {

	m := &webhookConfigManager{
		client:               client,
		kyvernoClient:        kyvernoClient,
		pInformer:            pInformer,
		npInformer:           npInformer,
		resCache:             resCache,
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmanager"),
		wildcardPolicy:       0,
		serverIP:             serverIP,
		autoUpdateWebhooks:   autoUpdateWebhooks,
		createDefaultWebhook: createDefaultWebhook,
		stopCh:               stopCh,
		log:                  log,
	}

	m.pLister = pInformer.Lister()
	m.npLister = npInformer.Lister()

	m.pListerSynced = pInformer.Informer().HasSynced
	m.npListerSynced = npInformer.Informer().HasSynced

	mutateCache, _ := m.resCache.GetGVRCache(kindMutating)
	m.mutateInformer = mutateCache.GetInformer()
	m.mutateInformerSynced = mutateCache.GetInformer().HasSynced

	validateCache, _ := m.resCache.GetGVRCache(kindValidating)
	m.validateInformer = validateCache.GetInformer()
	m.validateInformerSynced = validateCache.GetInformer().HasSynced

	return m
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

func (m *webhookConfigManager) addClusterPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	if hasWildcard(p) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}
	m.enqueue(p)
}

func (m *webhookConfigManager) updateClusterPolicy(old, cur interface{}) {
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	if hasWildcard(oldP) && !hasWildcard(curP) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	} else if !hasWildcard(oldP) && hasWildcard(curP) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}

	m.enqueue(curP)
}

func (m *webhookConfigManager) deleteClusterPolicy(obj interface{}) {
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

	if hasWildcard(p) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	}
	m.enqueue(p)
}

func (m *webhookConfigManager) addPolicy(obj interface{}) {
	p := obj.(*kyverno.Policy)
	if hasWildcard(p) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}

	pol := kyverno.ClusterPolicy(*p)
	m.enqueue(&pol)
}

func (m *webhookConfigManager) updatePolicy(old, cur interface{}) {
	oldP := old.(*kyverno.Policy)
	curP := cur.(*kyverno.Policy)

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	if hasWildcard(oldP) && !hasWildcard(curP) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	} else if !hasWildcard(oldP) && hasWildcard(curP) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}

	pol := kyverno.ClusterPolicy(*curP)
	m.enqueue(&pol)
}

func (m *webhookConfigManager) deletePolicy(obj interface{}) {
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
		m.log.V(4).Info("Recovered deleted ClusterPolicy '%s' from tombstone", "name", p.GetName())
	}

	if hasWildcard(p) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	}

	pol := kyverno.ClusterPolicy(*p)
	m.enqueue(&pol)
}

func (m *webhookConfigManager) deleteWebhook(obj interface{}) {
	m.log.WithName("deleteWebhook").Info("resource webhook configuration was deleted, recreating...")
	if webhook, ok := obj.(*unstructured.Unstructured); ok {
		k := webhook.GetKind()
		if (k == kindMutating && webhook.GetName() == config.MutatingWebhookConfigurationName) ||
			(k == kindValidating && webhook.GetName() == config.ValidatingWebhookConfigurationName) {
			m.enqueueAllPolicies()
		}
	}
}

func (m *webhookConfigManager) enqueueAllPolicies() {
	logger := m.log.WithName("enqueueAllPolicies")

	cpols, err := m.listPolicies("")
	if err != nil {
		logger.Error(err, "unabled to list clusterpolicies")
	}
	for _, cpol := range cpols {
		m.enqueue(cpol)
		logger.V(4).Info("added CLusterPolicy to the queue", "name", cpol.GetName())
	}

	nsCache, ok := m.resCache.GetGVRCache("Namespace")
	if !ok {
		nsCache, err = m.resCache.CreateGVKInformer("Namespace")
		if err != nil {
			logger.Error(err, "unabled to create Namespace listser")
			return
		}
	}

	namespaces, err := nsCache.Lister().List(labels.Everything())
	if err != nil {
		logger.Error(err, "unabled to list namespaces")
		return
	}

	for _, ns := range namespaces {
		pols, err := m.listPolicies(ns.GetName())
		if err != nil {
			logger.Error(err, "unabled to list policies", "namespace", ns.GetName())
		}

		for _, p := range pols {
			m.enqueue(p)
			logger.V(4).Info("added Policy to the queue", "namespace", p.GetName(), "name", p.GetName())
		}
	}
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

// start is a blocking call to configure webhook
func (m *webhookConfigManager) start() {
	defer utilruntime.HandleCrash()
	defer m.queue.ShutDown()

	m.log.Info("starting")
	defer m.log.Info("shutting down")

	if !cache.WaitForCacheSync(m.stopCh, m.pListerSynced, m.npListerSynced, m.mutateInformerSynced, m.validateInformerSynced) {
		m.log.Info("failed to sync informer cache")
		return
	}

	m.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.addClusterPolicy,
		UpdateFunc: m.updateClusterPolicy,
		DeleteFunc: m.deleteClusterPolicy,
	})

	m.npInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.addPolicy,
		UpdateFunc: m.updatePolicy,
		DeleteFunc: m.deletePolicy,
	})

	m.mutateInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: m.deleteWebhook,
	})

	m.validateInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: m.deleteWebhook,
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

	return m.reconcileWebhook(namespace, name)
}

func (m *webhookConfigManager) reconcileWebhook(namespace, name string) error {
	logger := m.log.WithName("reconcileWebhook").WithValues("namespace", namespace, "policy", name)

	policy, err := m.getPolicy(namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "unable to get policy object %s/%s", namespace, name)
	}

	ready := true
	// build webhook only if auto-update is enabled, otherwise directly update status to ready
	if m.autoUpdateWebhooks {
		webhooks, err := m.buildWebhooks(namespace)
		if err != nil {
			return err
		}

		if err := m.updateWebhookConfig(webhooks); err != nil {
			ready = false
			logger.Error(err, "failed to update webhook configurations for policy")
		}

		// DELETION of the policy
		if policy == nil {
			return nil
		}
	}

	if err := m.updateStatus(policy, ready); err != nil {
		return errors.Wrapf(err, "failed to update policy status %s/%s", namespace, name)
	}

	if ready {
		logger.Info("policy is ready to serve admission requests")
	}
	return nil
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

func (m *webhookConfigManager) listPolicies(namespace string) ([]*kyverno.ClusterPolicy, error) {
	if namespace != "" {
		polList, err := m.npLister.Policies(namespace).List(labels.Everything())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to list Policy")
		}

		policies := make([]*kyverno.ClusterPolicy, len(polList))
		for i, pol := range polList {
			p := kyverno.ClusterPolicy(*pol)
			policies[i] = &p
		}
		return policies, nil
	}

	cpolList, err := m.pLister.List(labels.Everything())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list ClusterPolicy")
	}

	return cpolList, nil
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

func (m *webhookConfigManager) buildWebhooks(namespace string) (res []*webhook, err error) {
	mutateIgnore := newWebhook(kindMutating, DefaultWebhookTimeout, kyverno.Ignore)
	mutateFail := newWebhook(kindMutating, DefaultWebhookTimeout, kyverno.Fail)
	validateIgnore := newWebhook(kindValidating, DefaultWebhookTimeout, kyverno.Ignore)
	validateFail := newWebhook(kindValidating, DefaultWebhookTimeout, kyverno.Fail)

	if atomic.LoadInt64(&m.wildcardPolicy) != 0 {
		for _, w := range []*webhook{mutateIgnore, mutateFail, validateIgnore, validateFail} {
			setWildcardConfig(w)
		}

		m.log.V(4).WithName("buildWebhooks").Info("warning: found wildcard policy, setting webhook configurations to accept admission requests of all kinds")
		return append(res, mutateIgnore, mutateFail, validateIgnore, validateFail), nil
	}

	policies, err := m.listPolicies(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "unable to list current policies")
	}

	for _, p := range policies {
		if p.HasValidate() || p.HasGenerate() {
			if p.Spec.FailurePolicy != nil && *p.Spec.FailurePolicy == kyverno.Ignore {
				m.mergeWebhook(validateIgnore, p, true)
			} else {
				m.mergeWebhook(validateFail, p, true)
			}
		}

		if p.HasMutate() || p.HasVerifyImages() {
			if p.Spec.FailurePolicy != nil && *p.Spec.FailurePolicy == kyverno.Ignore {
				m.mergeWebhook(mutateIgnore, p, false)
			} else {
				m.mergeWebhook(mutateFail, p, false)
			}
		}
	}

	res = append(res, mutateIgnore, mutateFail, validateIgnore, validateFail)
	return res, nil
}

func (m *webhookConfigManager) updateWebhookConfig(webhooks []*webhook) error {
	logger := m.log.WithName("updateWebhookConfig")
	webhooksMap := make(map[string]interface{}, len(webhooks))
	for _, w := range webhooks {
		key := webhookKey(w.kind, string(w.failurePolicy))
		webhooksMap[key] = w
	}

	var errs []string
	if err := m.compareAndUpdateWebhook(kindMutating, getResourceMutatingWebhookConfigName(m.serverIP), webhooksMap); err != nil {
		logger.V(4).Info("failed to update mutatingwebhookconfigurations", "error", err.Error())
		errs = append(errs, err.Error())
	}

	if err := m.compareAndUpdateWebhook(kindValidating, getResourceValidatingWebhookConfigName(m.serverIP), webhooksMap); err != nil {
		logger.V(4).Info("failed to update validatingwebhookconfigurations", "error", err.Error())
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (m *webhookConfigManager) getWebhook(webhookKind, webhookName string) (resourceWebhook *unstructured.Unstructured, err error) {
	get := func() error {
		webhookCache, _ := m.resCache.GetGVRCache(webhookKind)

		resourceWebhook, err = webhookCache.Lister().Get(webhookName)
		if err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "unable to get %s/%s", webhookKind, webhookName)
		} else if apierrors.IsNotFound(err) {
			m.createDefaultWebhook <- webhookKind
			return err
		}
		return nil
	}

	retryGetWebhook := common.RetryFunc(time.Second, 10*time.Second, get, m.log)
	if err := retryGetWebhook(); err != nil {
		return nil, err
	}

	return resourceWebhook, nil
}

func (m *webhookConfigManager) compareAndUpdateWebhook(webhookKind, webhookName string, webhooksMap map[string]interface{}) error {
	logger := m.log.WithName("compareAndUpdateWebhook").WithValues("kind", webhookKind, "name", webhookName)
	resourceWebhook, err := m.getWebhook(webhookKind, webhookName)
	if err != nil {
		return err
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

		newWebhook := webhooksMap[webhookKey(webhookKind, failurePolicy)]
		w, ok := newWebhook.(*webhook)
		if !ok {
			logger.Error(errors.New("type mismatched"), "expected *webhook, got %T", newWebooks)
			continue
		}

		if !reflect.DeepEqual(rules, []interface{}{w.rule}) {
			changed = true

			tmpRules, ok := newWebooks[i].(map[string]interface{})["rules"].([]interface{})
			if !ok {
				// init operations
				ops := []string{string(admregapi.Create), string(admregapi.Update), string(admregapi.Delete), string(admregapi.Connect)}
				if webhookKind == kindMutating {
					ops = []string{string(admregapi.Create), string(admregapi.Update)}
				}

				tmpRules = []interface{}{map[string]interface{}{}}
				if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), ops, "operations"); err != nil {
					return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiGroups)
				}
			}

			if w.rule == nil || reflect.DeepEqual(w.rule, map[string]interface{}{}) {
				// zero kyverno policy with the current failurePolicy, reset webhook rules to empty
				newWebooks[i].(map[string]interface{})["rules"] = []interface{}{}
				continue
			}

			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[apiGroups].([]string), apiGroups); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiGroups)
			}
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[apiVersions].([]string), apiVersions); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, apiVersions)
			}
			if err = unstructured.SetNestedStringSlice(tmpRules[0].(map[string]interface{}), w.rule[resources].([]string), resources); err != nil {
				return errors.Wrapf(err, "unable to set webhooks[%d].rules[0].%s", i, resources)
			}

			newWebooks[i].(map[string]interface{})["rules"] = tmpRules
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

func (m *webhookConfigManager) updateStatus(policy *kyverno.ClusterPolicy, status bool) error {
	policyCopy := policy.DeepCopy()
	policyCopy.Status.Ready = status
	if policy.GetNamespace() == "" {
		_, err := m.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), policyCopy, v1.UpdateOptions{})
		return err
	}

	_, err := m.kyvernoClient.KyvernoV1().Policies(policyCopy.GetNamespace()).UpdateStatus(context.TODO(), (*kyverno.Policy)(policyCopy), v1.UpdateOptions{})
	return err
}

// mergeWebhook merges the matching kinds of the policy to webhook.rule
func (m *webhookConfigManager) mergeWebhook(dst *webhook, policy *kyverno.ClusterPolicy, updateValidate bool) {
	matchedGVK := make([]string, 0)
	for _, rule := range policy.Spec.Rules {
		// matching kinds in generate policies need to be added to both webhook
		if rule.HasGenerate() {
			matchedGVK = append(matchedGVK, rule.MatchKinds()...)
			matchedGVK = append(matchedGVK, rule.Generation.ResourceSpec.Kind)
			continue
		}

		if (updateValidate && rule.HasValidate()) ||
			(!updateValidate && rule.HasMutate()) ||
			(!updateValidate && rule.HasVerifyImages()) {
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
			switch k {
			case "Binding":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/binding"})
			case "NodeProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes/proxy"})
			case "PodAttachOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/attach"})
			case "PodExecOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/exec"})
			case "PodPortForwardOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/portforward"})
			case "PodProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/proxy"})
			case "ServiceProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services/proxy"})
			default:
				_, gvr, err := m.client.DiscoveryClient.FindResource(gv, k)
				if err != nil {
					m.log.Error(err, "unable to convert GVK to GVR", "GVK", gvk)
					continue
				}
				gvrList = append(gvrList, gvr)
			}
		}
	}

	var groups, versions, rsrcs []string
	if val, ok := dst.rule[apiGroups]; ok {
		groups = make([]string, len(val.([]string)))
		copy(groups, val.([]string))
	}

	if val, ok := dst.rule[apiVersions]; ok {
		versions = make([]string, len(val.([]string)))
		copy(versions, val.([]string))
	}
	if val, ok := dst.rule[resources]; ok {
		rsrcs = make([]string, len(val.([]string)))
		copy(rsrcs, val.([]string))
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

func webhookKey(webhookKind, failurePolicy string) string {
	return strings.Join([]string{webhookKind, failurePolicy}, "/")
}

func hasWildcard(policy interface{}) bool {
	if p, ok := policy.(*kyverno.ClusterPolicy); ok {
		for _, rule := range p.Spec.Rules {
			if kinds := rule.MatchKinds(); utils.ContainsString(kinds, "*") {
				return true
			}
		}
	}

	if p, ok := policy.(*kyverno.Policy); ok {
		for _, rule := range p.Spec.Rules {
			if kinds := rule.MatchKinds(); utils.ContainsString(kinds, "*") {
				return true
			}
		}
	}
	return false
}

func setWildcardConfig(w *webhook) {
	w.rule[apiGroups] = []string{"*"}
	w.rule[apiVersions] = []string{"*"}
	w.rule[resources] = []string{"*/*"}
}
