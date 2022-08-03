package webhookconfig

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	"k8s.io/client-go/kubernetes"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var DefaultWebhookTimeout int32 = 10

// webhookConfigManager manges the webhook configuration dynamically
// it is NOT multi-thread safe
type webhookConfigManager struct {
	// clients
	discoveryClient dclient.IDiscovery
	kubeClient      kubernetes.Interface
	kyvernoClient   kyvernoclient.Interface

	// informers
	pInformer        kyvernov1informers.ClusterPolicyInformer
	npInformer       kyvernov1informers.PolicyInformer
	mutateInformer   admissionregistrationv1informers.MutatingWebhookConfigurationInformer
	validateInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer

	// listers
	pLister        kyvernov1listers.ClusterPolicyLister
	npLister       kyvernov1listers.PolicyLister
	mutateLister   admissionregistrationv1listers.MutatingWebhookConfigurationLister
	validateLister admissionregistrationv1listers.ValidatingWebhookConfigurationLister

	// queue
	queue workqueue.RateLimitingInterface

	// serverIP used to get the name of debug webhooks
	serverIP           string
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
	discoveryClient dclient.IDiscovery,
	kubeClient kubernetes.Interface,
	kyvernoClient kyvernoclient.Interface,
	pInformer kyvernov1informers.ClusterPolicyInformer,
	npInformer kyvernov1informers.PolicyInformer,
	mwcInformer admissionregistrationv1informers.MutatingWebhookConfigurationInformer,
	vwcInformer admissionregistrationv1informers.ValidatingWebhookConfigurationInformer,
	serverIP string,
	autoUpdateWebhooks bool,
	createDefaultWebhook chan<- string,
	stopCh <-chan struct{},
	log logr.Logger,
) manage {
	m := &webhookConfigManager{
		discoveryClient:      discoveryClient,
		kyvernoClient:        kyvernoClient,
		kubeClient:           kubeClient,
		pInformer:            pInformer,
		npInformer:           npInformer,
		mutateInformer:       mwcInformer,
		validateInformer:     vwcInformer,
		pLister:              pInformer.Lister(),
		npLister:             npInformer.Lister(),
		mutateLister:         mwcInformer.Lister(),
		validateLister:       vwcInformer.Lister(),
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configmanager"),
		wildcardPolicy:       0,
		serverIP:             serverIP,
		autoUpdateWebhooks:   autoUpdateWebhooks,
		createDefaultWebhook: createDefaultWebhook,
		stopCh:               stopCh,
		log:                  log,
	}

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
	p := obj.(*kyvernov1.ClusterPolicy)
	if hasWildcard(&p.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}
	m.enqueue(p)
}

func (m *webhookConfigManager) updateClusterPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}
	if hasWildcard(&oldP.Spec) && !hasWildcard(&curP.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	} else if !hasWildcard(&oldP.Spec) && hasWildcard(&curP.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}
	m.enqueue(curP)
}

func (m *webhookConfigManager) deleteClusterPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		// utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
		m.log.Info("Failed to get deleted object", "obj", obj)
		return
	}
	if hasWildcard(&p.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	}
	m.enqueue(p)
}

func (m *webhookConfigManager) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	if hasWildcard(&p.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}
	m.enqueue(p)
}

func (m *webhookConfigManager) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}
	if hasWildcard(&oldP.Spec) && !hasWildcard(&curP.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	} else if !hasWildcard(&oldP.Spec) && hasWildcard(&curP.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, int64(1))
	}
	m.enqueue(curP)
}

func (m *webhookConfigManager) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		// utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
		m.log.Info("Failed to get deleted object", "obj", obj)
		return
	}
	if hasWildcard(&p.Spec) {
		atomic.AddInt64(&m.wildcardPolicy, ^int64(0))
	}
	m.enqueue(p)
}

func (m *webhookConfigManager) deleteMutatingWebhook(obj interface{}) {
	m.log.WithName("deleteMutatingWebhook").Info("resource webhook configuration was deleted, recreating...")
	webhook, ok := kubeutils.GetObjectWithTombstone(obj).(*admissionregistrationv1.MutatingWebhookConfiguration)
	if !ok {
		m.log.Info("Failed to get deleted object", "obj", obj)
		return
	}
	if webhook.GetName() == config.MutatingWebhookConfigurationName {
		m.enqueueAllPolicies()
	}
}

func (m *webhookConfigManager) deleteValidatingWebhook(obj interface{}) {
	m.log.WithName("deleteMutatingWebhook").Info("resource webhook configuration was deleted, recreating...")
	webhook, ok := kubeutils.GetObjectWithTombstone(obj).(*admissionregistrationv1.ValidatingWebhookConfiguration)
	if !ok {
		m.log.Info("Failed to get deleted object", "obj", obj)
		return
	}
	if webhook.GetName() == config.ValidatingWebhookConfigurationName {
		m.enqueueAllPolicies()
	}
}

func (m *webhookConfigManager) enqueueAllPolicies() {
	logger := m.log.WithName("enqueueAllPolicies")
	policies, err := m.listAllPolicies()
	if err != nil {
		logger.Error(err, "unable to list policies")
	}
	for _, policy := range policies {
		m.enqueue(policy)
		logger.V(4).Info("added policy to the queue", "namespace", policy.GetNamespace(), "name", policy.GetName())
	}
}

func (m *webhookConfigManager) enqueue(policy interface{}) {
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

	m.mutateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: m.deleteMutatingWebhook,
	})

	m.validateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: m.deleteValidatingWebhook,
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

	_, err := m.getPolicy(namespace, name)
	isDeleted := apierrors.IsNotFound(err)
	if err != nil && !isDeleted {
		return errors.Wrapf(err, "unable to get policy object %s/%s", namespace, name)
	}

	ready := true
	var updateErr error
	// build webhook only if auto-update is enabled, otherwise directly update status to ready
	if m.autoUpdateWebhooks {
		webhooks, err := m.buildWebhooks(namespace)
		if err != nil {
			return err
		}

		if err := m.updateWebhookConfig(webhooks); err != nil {
			ready = false
			updateErr = errors.Wrapf(err, "failed to update webhook configurations for policy")
		}

		// DELETION of the policy
		if isDeleted {
			return nil
		}
	}

	if err := m.updateStatus(namespace, name, ready); err != nil {
		return errors.Wrapf(err, "failed to update policy status %s/%s", namespace, name)
	}

	if ready {
		logger.Info("policy is ready to serve admission requests")
	}
	return updateErr
}

func (m *webhookConfigManager) getPolicy(namespace, name string) (kyvernov1.PolicyInterface, error) {
	if namespace == "" {
		return m.pLister.Get(name)
	} else {
		return m.npLister.Policies(namespace).Get(name)
	}
}

func (m *webhookConfigManager) listAllPolicies() ([]kyvernov1.PolicyInterface, error) {
	policies := []kyvernov1.PolicyInterface{}
	polList, err := m.npLister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list Policy")
	}
	for _, p := range polList {
		policies = append(policies, p)
	}
	cpolList, err := m.pLister.List(labels.Everything())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list ClusterPolicy")
	}
	for _, p := range cpolList {
		policies = append(policies, p)
	}
	return policies, nil
}

func (m *webhookConfigManager) buildWebhooks(namespace string) (res []*webhook, err error) {
	mutateIgnore := newWebhook(kindMutating, DefaultWebhookTimeout, kyvernov1.Ignore)
	mutateFail := newWebhook(kindMutating, DefaultWebhookTimeout, kyvernov1.Fail)
	validateIgnore := newWebhook(kindValidating, DefaultWebhookTimeout, kyvernov1.Ignore)
	validateFail := newWebhook(kindValidating, DefaultWebhookTimeout, kyvernov1.Fail)

	if atomic.LoadInt64(&m.wildcardPolicy) != 0 {
		for _, w := range []*webhook{mutateIgnore, mutateFail, validateIgnore, validateFail} {
			setWildcardConfig(w)
		}

		m.log.V(4).WithName("buildWebhooks").Info("warning: found wildcard policy, setting webhook configurations to accept admission requests of all kinds")
		return append(res, mutateIgnore, mutateFail, validateIgnore, validateFail), nil
	}

	policies, err := m.listAllPolicies()
	if err != nil {
		return nil, errors.Wrap(err, "unable to list current policies")
	}

	for _, p := range policies {
		spec := p.GetSpec()
		if spec.HasValidate() || spec.HasGenerate() || spec.HasMutate() || spec.HasImagesValidationChecks() {
			if spec.GetFailurePolicy() == kyvernov1.Ignore {
				m.mergeWebhook(validateIgnore, p, true)
			} else {
				m.mergeWebhook(validateFail, p, true)
			}
		}

		if spec.HasMutate() || spec.HasVerifyImages() {
			if spec.GetFailurePolicy() == kyvernov1.Ignore {
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

	webhooksMap := map[string]*webhook{}
	for _, w := range webhooks {
		webhooksMap[webhookKey(w.kind, string(w.failurePolicy))] = w
	}

	var errs []string
	if err := m.updateMutatingWebhookConfiguration(getResourceMutatingWebhookConfigName(m.serverIP), webhooksMap); err != nil {
		logger.V(4).Info("failed to update mutatingwebhookconfigurations", "error", err.Error())
		errs = append(errs, err.Error())
	}

	if err := m.updateValidatingWebhookConfiguration(getResourceValidatingWebhookConfigName(m.serverIP), webhooksMap); err != nil {
		logger.V(4).Info("failed to update validatingwebhookconfigurations", "error", err.Error())
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (m *webhookConfigManager) updateMutatingWebhookConfiguration(webhookName string, webhooksMap map[string]*webhook) error {
	logger := m.log.WithName("updateMutatingWebhookConfiduration").WithValues("name", webhookName)
	resourceWebhook, err := m.mutateLister.Get(webhookName)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "unable to get %s/%s", kindMutating, webhookName)
	} else if apierrors.IsNotFound(err) {
		m.createDefaultWebhook <- kindMutating
		return err
	}
	for i := range resourceWebhook.Webhooks {
		newWebhook := webhooksMap[webhookKey(kindMutating, string(*resourceWebhook.Webhooks[i].FailurePolicy))]
		if newWebhook == nil || newWebhook.isEmpty() {
			resourceWebhook.Webhooks[i].Rules = []admissionregistrationv1.RuleWithOperations{}
		} else {
			resourceWebhook.Webhooks[i].TimeoutSeconds = &newWebhook.maxWebhookTimeout
			resourceWebhook.Webhooks[i].Rules = []admissionregistrationv1.RuleWithOperations{
				newWebhook.buildRuleWithOperations(admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete),
			}
		}
	}
	if _, err := m.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), resourceWebhook, metav1.UpdateOptions{}); err != nil {
		return errors.Wrapf(err, "unable to update: %s", resourceWebhook.GetName())
	}
	logger.V(4).Info("successfully updated the webhook configuration")
	return nil
}

func (m *webhookConfigManager) updateValidatingWebhookConfiguration(webhookName string, webhooksMap map[string]*webhook) error {
	logger := m.log.WithName("updateMutatingWebhookConfiduration").WithValues("name", webhookName)
	resourceWebhook, err := m.validateLister.Get(webhookName)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "unable to get %s/%s", kindValidating, webhookName)
	} else if apierrors.IsNotFound(err) {
		m.createDefaultWebhook <- kindValidating
		return err
	}
	for i := range resourceWebhook.Webhooks {
		newWebhook := webhooksMap[webhookKey(kindValidating, string(*resourceWebhook.Webhooks[i].FailurePolicy))]
		if newWebhook == nil || newWebhook.isEmpty() {
			resourceWebhook.Webhooks[i].Rules = []admissionregistrationv1.RuleWithOperations{}
		} else {
			resourceWebhook.Webhooks[i].TimeoutSeconds = &newWebhook.maxWebhookTimeout
			resourceWebhook.Webhooks[i].Rules = []admissionregistrationv1.RuleWithOperations{
				newWebhook.buildRuleWithOperations(admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete, admissionregistrationv1.Connect),
			}
		}
	}
	if _, err := m.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.TODO(), resourceWebhook, metav1.UpdateOptions{}); err != nil {
		return errors.Wrapf(err, "unable to update: %s", resourceWebhook.GetName())
	}
	logger.V(4).Info("successfully updated the webhook configuration")
	return nil
}

func (m *webhookConfigManager) updateStatus(namespace, name string, ready bool) error {
	update := func(meta *metav1.ObjectMeta, p kyvernov1.PolicyInterface, status *kyvernov1.PolicyStatus) bool {
		copy := status.DeepCopy()
		status.SetReady(ready)
		if toggle.AutogenInternals() {
			var rules []kyvernov1.Rule
			for _, rule := range autogen.ComputeRules(p) {
				if strings.HasPrefix(rule.Name, "autogen-") {
					rules = append(rules, rule)
				}
			}
			status.Autogen.Rules = rules
		} else {
			status.Autogen.Rules = nil
		}
		return !reflect.DeepEqual(status, copy)
	}
	if namespace == "" {
		p, err := m.pLister.Get(name)
		if err != nil {
			return err
		}
		if update(&p.ObjectMeta, p, &p.Status) {
			if _, err := m.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), p, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	} else {
		p, err := m.npLister.Policies(namespace).Get(name)
		if err != nil {
			return err
		}
		if update(&p.ObjectMeta, p, &p.Status) {
			if _, err := m.kyvernoClient.KyvernoV1().Policies(namespace).UpdateStatus(context.TODO(), p, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// webhook is the instance that aggregates the GVK of existing policies
// based on kind, failurePolicy and webhookTimeout
type webhook struct {
	kind              string
	maxWebhookTimeout int32
	failurePolicy     kyvernov1.FailurePolicyType
	groups            sets.String
	versions          sets.String
	resources         sets.String
}

func (wh *webhook) buildRuleWithOperations(ops ...admissionregistrationv1.OperationType) admissionregistrationv1.RuleWithOperations {
	return admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   wh.groups.List(),
			APIVersions: wh.versions.List(),
			Resources:   wh.resources.List(),
		},
		Operations: ops,
	}
}

func (wh *webhook) isEmpty() bool {
	return wh.groups.Len() == 0 || wh.versions.Len() == 0 || wh.resources.Len() == 0
}

// mergeWebhook merges the matching kinds of the policy to webhook.rule
func (m *webhookConfigManager) mergeWebhook(dst *webhook, policy kyvernov1.PolicyInterface, updateValidate bool) {
	matchedGVK := make([]string, 0)
	for _, rule := range autogen.ComputeRules(policy) {
		// matching kinds in generate policies need to be added to both webhook
		if rule.HasGenerate() {
			matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
			matchedGVK = append(matchedGVK, rule.Generation.ResourceSpec.Kind)
			continue
		}

		if (updateValidate && rule.HasValidate() || rule.HasImagesValidationChecks()) ||
			(updateValidate && rule.HasMutate() && rule.IsMutateExisting()) ||
			(!updateValidate && rule.HasMutate()) && !rule.IsMutateExisting() ||
			(!updateValidate && rule.HasVerifyImages()) {
			matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
		}
	}

	gvkMap := make(map[string]int)
	gvrList := make([]schema.GroupVersionResource, 0)
	for _, gvk := range matchedGVK {
		if _, ok := gvkMap[gvk]; !ok {
			gvkMap[gvk] = 1

			// note: webhook stores GVR in its rules while policy stores GVK in its rules definition
			gv, k := kubeutils.GetKindFromGVK(gvk)
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
				_, gvr, err := m.discoveryClient.FindResource(gv, k)
				if err != nil {
					m.log.Error(err, "unable to convert GVK to GVR", "GVK", gvk)
					continue
				}
				if strings.Contains(gvk, "*") {
					gvrList = append(gvrList, schema.GroupVersionResource{Group: gvr.Group, Version: "*", Resource: gvr.Resource})
				} else {
					m.log.V(4).Info("configuring webhook", "GVK", gvk, "GVR", gvr)
					gvrList = append(gvrList, gvr)
				}
			}
		}
	}

	for _, gvr := range gvrList {
		dst.groups.Insert(gvr.Group)
		dst.versions.Insert(gvr.Version)
		dst.resources.Insert(gvr.Resource)
	}

	if dst.resources.Has("pods") {
		dst.resources.Insert("pods/ephemeralcontainers")
	}
	if dst.resources.Has("services") {
		dst.resources.Insert("services/status")
	}

	spec := policy.GetSpec()
	if spec.WebhookTimeoutSeconds != nil {
		if dst.maxWebhookTimeout < *spec.WebhookTimeoutSeconds {
			dst.maxWebhookTimeout = *spec.WebhookTimeoutSeconds
		}
	}
}

func newWebhook(kind string, timeout int32, failurePolicy kyvernov1.FailurePolicyType) *webhook {
	return &webhook{
		kind:              kind,
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		groups:            sets.NewString(),
		versions:          sets.NewString(),
		resources:         sets.NewString(),
	}
}

func webhookKey(webhookKind, failurePolicy string) string {
	return strings.Join([]string{webhookKind, failurePolicy}, "/")
}

func hasWildcard(spec *kyvernov1.Spec) bool {
	for _, rule := range spec.Rules {
		if kinds := rule.MatchResources.GetKinds(); utils.ContainsString(kinds, "*") {
			return true
		}
	}
	return false
}

func setWildcardConfig(w *webhook) {
	w.groups = sets.NewString("*")
	w.versions = sets.NewString("*")
	w.resources = sets.NewString("*/*")
}
