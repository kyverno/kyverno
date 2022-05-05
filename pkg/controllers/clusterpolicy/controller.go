package clusterpolicy

import (
	"context"
	"fmt"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	admissionv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	"k8s.io/client-go/kubernetes"
	admissionv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

var (
	noneOnDryRun = admissionv1.SideEffectClassNoneOnDryRun
	ignore       = admissionv1.Ignore
	ifNeeded     = admissionv1.IfNeededReinvocationPolicy
)

type controller struct {
	// clients
	kubeClient      kubernetes.Interface
	discoveryClient discovery.DiscoveryInterface

	// listers
	clusterpolicyLister                  kyvernov1listers.ClusterPolicyLister
	mutatingwebhookconfigurationLister   admissionv1listers.MutatingWebhookConfigurationLister
	validatingwebhookconfigurationLister admissionv1listers.ValidatingWebhookConfigurationLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	clusterpolicyInformer kyvernov1informers.ClusterPolicyInformer,
	mutatingwebhookconfigurationInformer admissionv1informers.MutatingWebhookConfigurationInformer,
	validatingwebhookconfigurationInformer admissionv1informers.ValidatingWebhookConfigurationInformer,
) *controller {
	c := controller{
		kubeClient:                           kubeClient,
		discoveryClient:                      memory.NewMemCacheClient(kubeClient.Discovery()),
		clusterpolicyLister:                  clusterpolicyInformer.Lister(),
		mutatingwebhookconfigurationLister:   mutatingwebhookconfigurationInformer.Lister(),
		validatingwebhookconfigurationLister: validatingwebhookconfigurationInformer.Lister(),
		queue:                                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "clusterpolicy-controller"),
	}
	clusterpolicyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.add,
		UpdateFunc: c.update,
		DeleteFunc: c.delete,
	})
	return &c
}

func (c *controller) add(obj interface{}) {
	c.enqueue(obj.(*kyvernov1.ClusterPolicy))
}

func (c *controller) update(old, cur interface{}) {
	c.enqueue(cur.(*kyvernov1.ClusterPolicy))
}

func (c *controller) delete(obj interface{}) {
	cm, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if ok {
		c.enqueue(cm)
	} else {
		logger.Info("Failed to get deleted object", "obj", obj)
	}
}

func (c *controller) enqueue(obj *kyvernov1.ClusterPolicy) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		logger.Error(err, "failed to compute key name")
	} else {
		c.queue.Add(key)
	}
}

func (c *controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
	} else if errors.IsNotFound(err) {
		logger.V(4).Info("Dropping update request from the queue", "key", key, "error", err.Error())
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying update request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
	} else {
		logger.Error(err, "failed to process update request", "key", key)
		c.queue.Forget(key)
	}
}

func (c *controller) processNextWorkItem() bool {
	if key, quit := c.queue.Get(); !quit {
		defer c.queue.Done(key)
		c.handleErr(c.reconcile(key.(string)), key)
		return true
	}
	return false
}

func (c *controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *controller) reconcile(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	logger := logger.WithValues("key", key, "name", name)
	logger.Info("reconciling ...")
	if err != nil {
		return err
	}
	if clusterPolicy, err := c.clusterpolicyLister.Get(name); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// TODO: we could use owner references here
		return c.deleteClusterPolicyWebhooks(name)
	} else {
		return c.createOrUpdateClusterPolicyWebhooks(clusterPolicy)
	}
	// TODO: update policy status
}

func (c *controller) deleteClusterPolicyWebhooks(name string) error {
	name = fmt.Sprintf("cpol-%s", name)
	if err := c.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if err := c.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *controller) createOrUpdateClusterPolicyWebhooks(cpol *kyvernov1.ClusterPolicy) error {
	// compute desired state
	name := fmt.Sprintf("cpol-%s", cpol.GetName())
	mutation, err := c.buildMutatingWebhookConfiguration(name, cpol)
	if err != nil {
		return err
	}
	validation, err := c.buildValidatingWebhookConfiguration(name, cpol)
	if err != nil {
		return err
	}
	// make changes
	if mutation.ResourceVersion == "" {
		_, err = c.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), mutation, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = c.kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), mutation, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	if validation.ResourceVersion == "" {
		_, err = c.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), validation, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = c.kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.TODO(), validation, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) buildMutatingWebhookConfiguration(name string, cpol *kyvernov1.ClusterPolicy) (*admissionv1.MutatingWebhookConfiguration, error) {
	mutation, err := c.mutatingwebhookconfigurationLister.Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		mutation = &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}
	mutation.Webhooks = []admissionv1.MutatingWebhook{}
	path := fmt.Sprintf("%s/%s", config.MutatingWebhookServicePath, cpol.Name)
	for _, rule := range autogen.ComputeRules(cpol) {
		groups, versions, resources := getRule(c.discoveryClient, rule, true)
		mutation.Webhooks = append(mutation.Webhooks, admissionv1.MutatingWebhook{
			Name: fmt.Sprintf("%s.kyverno.io", rule.Name),
			Rules: []admissionv1.RuleWithOperations{{
				Rule: admissionv1.Rule{
					APIGroups:   groups,
					APIVersions: versions,
					Resources:   resources,
				},
				Operations: []admissionv1.OperationType{
					admissionv1.OperationAll,
				},
			}},
			ClientConfig: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{
					Namespace: config.KyvernoNamespace,
					Name:      config.KyvernoServiceName,
					Path:      &path,
				},
				// TODO
				// CABundle: caData,
			},
			FailurePolicy:           &ignore,
			SideEffects:             &noneOnDryRun,
			ReinvocationPolicy:      &ifNeeded,
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
		})
	}
	return mutation, nil
}

func (c *controller) buildValidatingWebhookConfiguration(name string, cpol *kyvernov1.ClusterPolicy) (*admissionv1.ValidatingWebhookConfiguration, error) {
	validation, err := c.validatingwebhookconfigurationLister.Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		validation = &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}
	validation.Webhooks = []admissionv1.ValidatingWebhook{}
	path := fmt.Sprintf("%s/%s", config.ValidatingWebhookServicePath, cpol.Name)
	for _, rule := range autogen.ComputeRules(cpol) {
		groups, versions, resources := getRule(c.discoveryClient, rule, true)
		validation.Webhooks = append(validation.Webhooks, admissionv1.ValidatingWebhook{
			Name: fmt.Sprintf("%s.kyverno.io", rule.Name),
			Rules: []admissionv1.RuleWithOperations{{
				Rule: admissionv1.Rule{
					APIGroups:   groups,
					APIVersions: versions,
					Resources:   resources,
				},
				Operations: []admissionv1.OperationType{
					admissionv1.OperationAll,
				},
			}},
			ClientConfig: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{
					Namespace: config.KyvernoNamespace,
					Name:      config.KyvernoServiceName,
					Path:      &path,
				},
				// TODO
				// CABundle: caData,
			},
			FailurePolicy:           &ignore,
			SideEffects:             &noneOnDryRun,
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
		})
	}
	return validation, nil
}
