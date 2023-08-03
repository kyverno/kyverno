package vapgeneration

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/controllers"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	"k8s.io/client-go/kubernetes"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 2
	ControllerName = "vap-generation-controller"
	maxRetries     = 10
)

type controller struct {
	// clients
	client        kubernetes.Interface
	kyvernoClient versioned.Interface

	// listers
	cpolLister       kyvernov1listers.ClusterPolicyLister
	vapLister        admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyLister
	vapbindingLister admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyBindingLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client kubernetes.Interface,
	kyvernoClient versioned.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vapInformer admissionregistrationv1alpha1informers.ValidatingAdmissionPolicyInformer,
	vapbindingInformer admissionregistrationv1alpha1informers.ValidatingAdmissionPolicyBindingInformer,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := &controller{
		client:           client,
		kyvernoClient:    kyvernoClient,
		cpolLister:       cpolInformer.Lister(),
		vapLister:        vapInformer.Lister(),
		vapbindingLister: vapbindingInformer.Lister(),
		queue:            queue,
	}

	// Set up an event handler for when Kyverno policies change
	controllerutils.AddEventHandlersT(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)

	// Set up an event handler for when validating admission policies change
	controllerutils.AddEventHandlersT(vapInformer.Informer(), c.addVAP, c.updateVAP, c.deleteVAP)

	// Set up an event handler for when validating admission policy bindings change
	controllerutils.AddEventHandlersT(vapbindingInformer.Informer(), c.addVAPbinding, c.updateVAPbinding, c.deleteVAPbinding)

	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) addPolicy(obj kyvernov1.PolicyInterface) {
	logger.Info("policy created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueuePolicy(obj)
}

func (c *controller) updatePolicy(old, obj kyvernov1.PolicyInterface) {
	if datautils.DeepEqual(old.GetSpec(), obj.GetSpec()) {
		return
	}
	logger.Info("policy updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueuePolicy(obj)
}

func (c *controller) deletePolicy(obj kyvernov1.PolicyInterface) {
	var p kyvernov1.PolicyInterface

	switch kubeutils.GetObjectWithTombstone(obj).(type) {
	case *kyvernov1.ClusterPolicy:
		p = kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	default:
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.Info("policy deleted", "uid", p.GetUID(), "kind", p.GetKind(), "name", p.GetName())
	c.enqueuePolicy(obj)
}

func (c *controller) enqueuePolicy(obj kyvernov1.PolicyInterface) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract policy name")
		return
	}
	c.queue.Add(key)
}

func (c *controller) addVAP(obj *v1alpha1.ValidatingAdmissionPolicy) {
	c.enqueueVAP(obj)
}

func (c *controller) updateVAP(old, obj *v1alpha1.ValidatingAdmissionPolicy) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	c.enqueueVAP(obj)
}

func (c *controller) deleteVAP(obj *v1alpha1.ValidatingAdmissionPolicy) {
	c.enqueueVAP(obj)
}

func (c *controller) enqueueVAP(v *v1alpha1.ValidatingAdmissionPolicy) {
	if len(v.OwnerReferences) == 1 {
		if v.OwnerReferences[0].Kind == "ClusterPolicy" {
			cpol, err := c.cpolLister.Get(v.OwnerReferences[0].Name)
			if err != nil {
				return
			}
			c.enqueuePolicy(cpol)
		}
	}
}

func (c *controller) addVAPbinding(obj *v1alpha1.ValidatingAdmissionPolicyBinding) {
	c.enqueueVAPbinding(obj)
}

func (c *controller) updateVAPbinding(old, obj *v1alpha1.ValidatingAdmissionPolicyBinding) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	c.enqueueVAPbinding(obj)
}

func (c *controller) deleteVAPbinding(obj *v1alpha1.ValidatingAdmissionPolicyBinding) {
	c.enqueueVAPbinding(obj)
}

func (c *controller) enqueueVAPbinding(vb *v1alpha1.ValidatingAdmissionPolicyBinding) {
	if len(vb.OwnerReferences) == 1 {
		if vb.OwnerReferences[0].Kind == "ClusterPolicy" {
			cpol, err := c.cpolLister.Get(vb.OwnerReferences[0].Name)
			if err != nil {
				return
			}
			c.enqueuePolicy(cpol)
		}
	}
}

func (c *controller) getClusterPolicy(name string) (kyvernov1.PolicyInterface, error) {
	cpolicy, err := c.cpolLister.Get(name)
	if err != nil {
		return nil, err
	}
	return cpolicy, nil
}

func (c *controller) getValidatingAdmissionPolicy(name string) (*v1alpha1.ValidatingAdmissionPolicy, error) {
	vap, err := c.vapLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vap, nil
}

func (c *controller) getValidatingAdmissionPolicyBinding(name string) (*v1alpha1.ValidatingAdmissionPolicyBinding, error) {
	vapbinding, err := c.vapbindingLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vapbinding, nil
}

func (c *controller) buildValidatingAdmissionPolicy(vap *v1alpha1.ValidatingAdmissionPolicy, cpol kyvernov1.PolicyInterface) error {
	// set owner reference
	vap.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "kyverno.io/v1",
			Kind:       cpol.GetKind(),
			Name:       cpol.GetName(),
			UID:        cpol.GetUID(),
		},
	}

	// get kinds from Kyverno rule
	var resourceRules []v1alpha1.NamedRuleWithOperations
	rule := cpol.GetSpec().Rules[0]
	kinds := rule.MatchResources.GetKinds()
	for _, kind := range kinds {
		group, version, resource, _ := kubeutils.ParseKindSelector(kind)
		resourceRule := v1alpha1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					Resources:   []string{strings.ToLower(resource) + "s"},
					APIGroups:   []string{group},
					APIVersions: []string{version},
				},
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				},
			},
		}
		resourceRules = append(resourceRules, resourceRule)
	}

	// set validating admission policy spec
	vap.Spec = v1alpha1.ValidatingAdmissionPolicySpec{
		ParamKind: rule.Validation.CEL.ParamKind,
		MatchConstraints: &v1alpha1.MatchResources{
			ResourceRules: resourceRules,
		},
		Validations:      rule.Validation.CEL.Expressions,
		AuditAnnotations: rule.Validation.CEL.AuditAnnotations,
	}

	// set labels
	controllerutils.SetManagedByKyvernoLabel(vap)
	return nil
}

func (c *controller) buildValidatingAdmissionPolicyBinding(vapbinding *v1alpha1.ValidatingAdmissionPolicyBinding, cpol kyvernov1.PolicyInterface) error {
	// set owner reference
	vapbinding.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "kyverno.io/v1",
			Kind:       cpol.GetKind(),
			Name:       cpol.GetName(),
			UID:        cpol.GetUID(),
		},
	}

	// set validation action for vap binding
	var validationActions []v1alpha1.ValidationAction
	action := cpol.GetSpec().ValidationFailureAction
	if action.Enforce() {
		validationActions = append(validationActions, v1alpha1.Deny)
	} else if action.Audit() {
		validationActions = append(validationActions, v1alpha1.Audit)
		validationActions = append(validationActions, v1alpha1.Warn)
	}

	// set validating admission policy binding spec
	rule := cpol.GetSpec().Rules[0]
	vapbinding.Spec = v1alpha1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName:        cpol.GetName(),
		ParamRef:          rule.Validation.CEL.ParamRef,
		ValidationActions: validationActions,
	}

	// set labels
	controllerutils.SetManagedByKyvernoLabel(vapbinding)
	return nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.getClusterPolicy(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "unable to get the policy from policy informer")
		return err
	}

	spec := policy.GetSpec()
	generate := canGenerateVAP(spec)
	if !generate {
		return nil
	}

	polName := policy.GetName()
	observedVAP, err := c.getValidatingAdmissionPolicy(polName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		observedVAP = &v1alpha1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: polName,
			},
		}
	}

	observedVAPbinding, err := c.getValidatingAdmissionPolicyBinding(polName + "-binding")
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		observedVAPbinding = &v1alpha1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: polName + "-binding",
			},
		}
	}

	if observedVAP.ResourceVersion == "" {
		err := c.buildValidatingAdmissionPolicy(observedVAP, policy)
		if err != nil {
			return err
		}
		_, err = c.client.AdmissionregistrationV1alpha1().ValidatingAdmissionPolicies().Create(ctx, observedVAP, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = controllerutils.Update(
			ctx,
			observedVAP,
			c.client.AdmissionregistrationV1alpha1().ValidatingAdmissionPolicies(),
			func(observed *v1alpha1.ValidatingAdmissionPolicy) error {
				return c.buildValidatingAdmissionPolicy(observed, policy)
			})
		if err != nil {
			return err
		}
	}

	if observedVAPbinding.ResourceVersion == "" {
		err := c.buildValidatingAdmissionPolicyBinding(observedVAPbinding, policy)
		if err != nil {
			return err
		}
		_, err = c.client.AdmissionregistrationV1alpha1().ValidatingAdmissionPolicyBindings().Create(ctx, observedVAPbinding, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = controllerutils.Update(
			ctx,
			observedVAPbinding,
			c.client.AdmissionregistrationV1alpha1().ValidatingAdmissionPolicyBindings(),
			func(observed *v1alpha1.ValidatingAdmissionPolicyBinding) error {
				return c.buildValidatingAdmissionPolicyBinding(observed, policy)
			})
		if err != nil {
			return err
		}
	}
	return nil
}
