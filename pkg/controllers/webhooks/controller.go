package webhooks

import (
	"context"
	"reflect"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	adminformers "k8s.io/client-go/informers/admissionregistration/v1"
	admlisters "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var DefaultWebhookTimeout int64 = 10

type Controller interface {
	Run(stopCh <-chan struct{})
}

type controller struct {
	// clients
	client        *client.Client
	kyvernoClient kyvernoclient.Interface

	// informers
	pInformer        kyvernoinformer.ClusterPolicyInformer
	npInformer       kyvernoinformer.PolicyInformer
	mutateInformer   adminformers.MutatingWebhookConfigurationInformer
	validateInformer adminformers.ValidatingWebhookConfigurationInformer

	// listers
	pLister        kyvernolister.ClusterPolicyLister
	npLister       kyvernolister.PolicyLister
	mutateLister   admlisters.MutatingWebhookConfigurationLister
	validateLister admlisters.ValidatingWebhookConfigurationLister

	// queue
	queue workqueue.RateLimitingInterface

	// serverIP used to get the name of debug webhooks
	serverIP string

	autoUpdateWebhooks bool

	createDefaultWebhook chan<- string
}

func NewController(
	client *client.Client,
	kyvernoClient *kyvernoclient.Clientset,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	mwcInformer adminformers.MutatingWebhookConfigurationInformer,
	vwcInformer adminformers.ValidatingWebhookConfigurationInformer,
	serverIP string,
	autoUpdateWebhooks bool,
) Controller {
	m := &controller{
		client:             client,
		kyvernoClient:      kyvernoClient,
		pInformer:          pInformer,
		npInformer:         npInformer,
		mutateInformer:     mwcInformer,
		validateInformer:   vwcInformer,
		pLister:            pInformer.Lister(),
		npLister:           npInformer.Lister(),
		mutateLister:       mwcInformer.Lister(),
		validateLister:     vwcInformer.Lister(),
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "webhooks-controller"),
		serverIP:           serverIP,
		autoUpdateWebhooks: autoUpdateWebhooks,
		// createDefaultWebhook: createDefaultWebhook,
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
	m.mutateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: m.deleteMutatingWebhook,
	})
	m.validateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: m.deleteValidatingWebhook,
	})
	return m
}

func (m *controller) deleteMutatingWebhook(obj interface{}) {
	m.enqueue()
}

func (m *controller) deleteValidatingWebhook(obj interface{}) {
	m.enqueue()
}

func (m *controller) addClusterPolicy(obj interface{}) {
	m.enqueue()
}

func (m *controller) updateClusterPolicy(old, cur interface{}) {
	m.enqueue()
}

func (m *controller) deleteClusterPolicy(obj interface{}) {
	m.enqueue()
}

func (m *controller) addPolicy(obj interface{}) {
	m.enqueue()
}

func (m *controller) updatePolicy(old, cur interface{}) {
	m.enqueue()
}

func (m *controller) deletePolicy(obj interface{}) {
	m.enqueue()
}

func (m *controller) enqueue() {
	m.queue.Add("dummy")
}

func (m *controller) handleErr(err error, key interface{}) {
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

func (m *controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer m.queue.ShutDown()
	defer logger.Info("shutting down")
	logger.Info("starting")
	go wait.Until(m.worker, time.Second, stopCh)
	<-stopCh
}

func (pc *controller) worker() {
	for pc.processNextWorkItem() {
	}
}

func (m *controller) processNextWorkItem() bool {
	key, quit := m.queue.Get()
	if quit {
		return false
	}
	defer m.queue.Done(key)
	err := m.reconcile()
	m.handleErr(err, key)
	return true
}

func (m *controller) reconcile() error {
	logger.Info("reconciling ...")
	defer logger.Info("reconciliation done.")
	// list policies
	policies, err := m.listAllPolicies()
	if err != nil {
		return errors.Wrap(err, "unable to list current policies")
	}
	ready := true
	if m.autoUpdateWebhooks {
		// build webhook only if auto-update is enabled, otherwise directly update status to ready
		webhooks, err := m.buildWebhooks(policies)
		if err != nil {
			return err
		}
		// compare actual againt desired state and update if necessary
		if err := m.updateWebhookConfig(webhooks); err != nil {
			logger.Error(err, "failed to update webhook configurations for policy")
			ready = false
		}
	}
	// update policy status
	for _, p := range policies {
		namespace, name := p.GetNamespace(), p.GetName()
		if err := m.updateStatus(namespace, name, ready); err != nil {
			return errors.Wrapf(err, "failed to update policy status %s/%s", namespace, name)
		}
	}

	return nil
}

func (m *controller) listAllPolicies() ([]kyverno.PolicyInterface, error) {
	policies := []kyverno.PolicyInterface{}
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

func (m *controller) buildWebhooks(policies []kyverno.PolicyInterface) ([]*webhook, error) {
	var res []*webhook
	mutateIgnore := newWebhook(kindMutating, DefaultWebhookTimeout, kyverno.Ignore)
	mutateFail := newWebhook(kindMutating, DefaultWebhookTimeout, kyverno.Fail)
	validateIgnore := newWebhook(kindValidating, DefaultWebhookTimeout, kyverno.Ignore)
	validateFail := newWebhook(kindValidating, DefaultWebhookTimeout, kyverno.Fail)
	wildcard := false
	for _, p := range policies {
		if !wildcard {
			wildcard = hasWildcard(p.GetSpec())
		}
	}
	if wildcard {
		for _, w := range []*webhook{mutateIgnore, mutateFail, validateIgnore, validateFail} {
			setWildcardConfig(w)
		}
		logger.V(4).WithName("buildWebhooks").Info("warning: found wildcard policy, setting webhook configurations to accept admission requests of all kinds")
		return append(res, mutateIgnore, mutateFail, validateIgnore, validateFail), nil
	}
	for _, p := range policies {
		spec := p.GetSpec()
		if spec.HasValidate() || spec.HasGenerate() || spec.HasMutate() || spec.HasImagesValidationChecks() {
			if spec.GetFailurePolicy() == kyverno.Ignore {
				mergeWebhook(validateIgnore, p, m.client.DiscoveryClient, true)
			} else {
				mergeWebhook(validateFail, p, m.client.DiscoveryClient, true)
			}
		}
		if spec.HasMutate() || spec.HasVerifyImages() {
			if spec.GetFailurePolicy() == kyverno.Ignore {
				mergeWebhook(mutateIgnore, p, m.client.DiscoveryClient, false)
			} else {
				mergeWebhook(mutateFail, p, m.client.DiscoveryClient, false)
			}
		}
	}
	res = append(res, mutateIgnore, mutateFail, validateIgnore, validateFail)
	return res, nil
}

func (m *controller) updateWebhookConfig(webhooks []*webhook) error {
	logger := logger.WithName("updateWebhookConfig")

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

func (m *controller) updateStatus(namespace, name string, ready bool) error {
	update := func(meta *metav1.ObjectMeta, spec *kyverno.Spec, status *kyverno.PolicyStatus) bool {
		copy := status.DeepCopy()
		requested, _, activated := autogen.GetControllers(meta, spec)
		status.SetReady(ready)
		status.Autogen.Requested = requested
		status.Autogen.Activated = activated
		status.Rules = spec.Rules
		return !reflect.DeepEqual(status, copy)
	}
	if namespace == "" {
		p, err := m.pLister.Get(name)
		if err != nil {
			return err
		}
		if update(&p.ObjectMeta, &p.Spec, &p.Status) {
			if _, err := m.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), p, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	} else {
		p, err := m.npLister.Policies(namespace).Get(name)
		if err != nil {
			return err
		}
		if update(&p.ObjectMeta, &p.Spec, &p.Status) {
			if _, err := m.kyvernoClient.KyvernoV1().Policies(namespace).UpdateStatus(context.TODO(), p, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasWildcard(spec *kyverno.Spec) bool {
	for _, rule := range spec.Rules {
		if kinds := rule.MatchResources.GetKinds(); utils.ContainsString(kinds, "*") {
			return true
		}
	}
	return false
}
