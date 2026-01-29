package policystatus

import (
	"context"
	baseerrors "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	auth "github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/webhook"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	ControllerName string = "status-controller"
	Workers        int    = 3
	maxRetries     int    = 3
)

type Controller interface {
	controllers.Controller
}

type controller struct {
	dclient          dclient.Interface
	client           versioned.Interface
	queue            workqueue.TypedRateLimitingInterface[any]
	authChecker      auth.AuthChecker
	polStateRecorder webhook.StateRecorder
}

func NewController(
	dclient dclient.Interface,
	client versioned.Interface,
	vpolInformer policiesv1beta1informers.ValidatingPolicyInformer,
	nvpolInformer policiesv1beta1informers.NamespacedValidatingPolicyInformer,
	ivpolInformer policiesv1beta1informers.ImageValidatingPolicyInformer,
	nivpolInformer policiesv1beta1informers.NamespacedImageValidatingPolicyInformer,
	mpolInformer policiesv1beta1informers.MutatingPolicyInformer,
	nmpolInformer policiesv1beta1informers.NamespacedMutatingPolicyInformer,
	gpolInformer policiesv1beta1informers.GeneratingPolicyInformer,
	reportsSA string,
	polStateRecorder webhook.StateRecorder,
) Controller {
	c := &controller{
		dclient: dclient,
		client:  client,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName}),
		authChecker:      auth.NewSubjectChecker(dclient.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), reportsSA, nil),
		polStateRecorder: polStateRecorder,
	}

	enqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(controllerutils.MetaNamespaceKey, controllerutils.Queue(c.queue)))
	_, err := controllerutils.AddEventHandlers(
		vpolInformer.Informer(),
		controllerutils.AddFunc(logger, enqueueFunc),
		controllerutils.UpdateFunc(logger, enqueueFunc),
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for webhook state recorder")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		vpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			vpol, ok := obj.(*policiesv1beta1.ValidatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.ValidatingPolicyType, vpol.Name, ""))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for ValidatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		nvpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			nvpol, ok := obj.(*policiesv1beta1.NamespacedValidatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.NamespacedValidatingPolicyType, nvpol.Name, nvpol.Namespace))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for NamespacedValidatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		ivpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			ivpol, ok := obj.(*policiesv1beta1.ImageValidatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.ImageValidatingPolicyType, ivpol.Name, ""))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for ImageValidatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		nivpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			nivpol, ok := obj.(*policiesv1beta1.NamespacedImageValidatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.NamespacedImageValidatingPolicyType, nivpol.Name, nivpol.Namespace))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for NamespacedImageValidatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		mpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			mpol, ok := obj.(*policiesv1beta1.MutatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.MutatingPolicyType, mpol.Name, ""))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for MutatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		nmpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			nmpol, ok := obj.(*policiesv1beta1.NamespacedMutatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.MutatingPolicyType, nmpol.Name, nmpol.Namespace))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for NamespacedMutatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		gpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			gpol, ok := obj.(*policiesv1beta1.GeneratingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.GeneratingPolicyType, gpol.Name, ""))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for GeneratingPolicy")
	}
	return c
}

func (c controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.watchdog)
}

func (c *controller) watchdog(ctx context.Context, logger logr.Logger) {
	notifyChan := c.polStateRecorder.(*webhook.Recorder).NotifyChan
	for key := range notifyChan {
		c.queue.Add(key)
	}
}

func (c controller) reconcile(ctx context.Context, logger logr.Logger, key string, _ string, _ string) error {
	polType, name, namespace := webhook.ParseRecorderKey(key)
	if polType == webhook.ValidatingPolicyType {
		vpol, err := c.client.PoliciesV1beta1().ValidatingPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("validating policy not found", "name", name)
				return nil
			}
			return err
		}

		return c.updateVpolStatus(ctx, vpol)
	}
	if polType == webhook.NamespacedValidatingPolicyType {
		nvpol, err := c.client.PoliciesV1beta1().NamespacedValidatingPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("namespaced validating policy not found", "name", name, "namespace", namespace)
				return nil
			}
			return err
		}
		return c.updateNVpolStatus(ctx, nvpol)
	}
	if polType == webhook.ImageValidatingPolicyType {
		ivpol, err := c.client.PoliciesV1beta1().ImageValidatingPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("imageVerification policy not found", "name", name)
				return nil
			}
			return err
		}
		return c.updateIvpolStatus(ctx, ivpol)
	}

	if polType == webhook.NamespacedImageValidatingPolicyType {
		nivpol, err := c.client.PoliciesV1beta1().NamespacedImageValidatingPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("namespaced image verification policy not found", "name", name, "namespace", namespace)
				return nil
			}
			return err
		}
		return c.updateNivpolStatus(ctx, nivpol)
	}

	if polType == webhook.MutatingPolicyType {
		mpol, err := c.client.PoliciesV1beta1().MutatingPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("mutating policy not found", "name", name)
				return nil
			}
			return err
		}
		return c.updateMpolStatus(ctx, mpol)
	}

	if polType == webhook.NamespacedMutatingPolicyType {
		nmpol, err := c.client.PoliciesV1beta1().NamespacedMutatingPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("namespaced mutating policy not found", "name", name, "namespace", namespace)
				return nil
			}
			return err
		}
		return c.updateNMpolStatus(ctx, nmpol)
	}

	if polType == webhook.GeneratingPolicyType {
		gpol, err := c.client.PoliciesV1beta1().GeneratingPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("generating policy not found", "name", name)
				return nil
			}
			return err
		}
		return c.updateGpolStatus(ctx, gpol)
	}
	return nil
}

func (c controller) reconcileConditions(ctx context.Context, policy engineapi.GenericPolicy) *policiesv1beta1.ConditionStatus {
	var key string
	var matchConstraints admissionregistrationv1.MatchResources
	status := &policiesv1beta1.ConditionStatus{}
	backgroundOnly := false
	switch policy.GetKind() {
	case webhook.MutatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.MutatingPolicyType, policy.GetName(), "")
		matchConstraints = policy.AsMutatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsMutatingPolicy().GetSpec().AdmissionEnabled() && policy.AsMutatingPolicy().GetSpec().BackgroundEnabled())
		// MutatingPolicy uses v1beta1.ConditionStatus, convert to return type
		v1beta1Status := policy.AsMutatingPolicy().GetStatus().ConditionStatus
		status = &v1beta1Status
	case webhook.GeneratingPolicyType:
		key = webhook.BuildRecorderKey(webhook.GeneratingPolicyType, policy.GetName(), "")
		matchConstraints = policy.AsGeneratingPolicy().GetMatchConstraints()
		// GeneratingPolicy uses v1alpha1.ConditionStatus, convert to v1beta1
		v1alpha1Status := policy.AsGeneratingPolicy().GetStatus().ConditionStatus
		status = &policiesv1beta1.ConditionStatus{
			Conditions: v1alpha1Status.Conditions,
			Ready:      v1alpha1Status.Ready,
			Message:    v1alpha1Status.Message,
		}
	}

	if !backgroundOnly {
		if ready, ok := c.polStateRecorder.Ready(key); ready {
			status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "Webhook configured.")
		} else if ok {
			status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "Policy is not configured in the webhook.")
		}
	}

	gvrs := c.resolveGVRs(matchConstraints.ResourceRules)
	errs := c.permissionsCheck(ctx, gvrs)
	if errs != nil {
		status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionFalse, fmt.Sprintf("Policy is not ready for reporting, missing permissions: %v.", multierr.Combine(errs...)))
	} else {
		status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionTrue, "Policy is ready for reporting.")
	}
	return status
}

func (c controller) reconcileBeta1Conditions(ctx context.Context, policy engineapi.GenericPolicy) *policiesv1beta1.ConditionStatus {
	var key string
	var matchConstraints admissionregistrationv1.MatchResources
	status := &policiesv1beta1.ConditionStatus{}
	backgroundOnly := false
	switch policy.GetKind() {
	case webhook.ImageValidatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.ImageValidatingPolicyType, policy.GetName(), "")
		matchConstraints = policy.AsImageValidatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsImageValidatingPolicy().GetSpec().AdmissionEnabled() && policy.AsImageValidatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsImageValidatingPolicy().GetStatus().ConditionStatus
	case webhook.NamespacedImageValidatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.NamespacedImageValidatingPolicyType, policy.GetName(), policy.GetNamespace())
		matchConstraints = policy.AsNamespacedImageValidatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsNamespacedImageValidatingPolicy().GetSpec().AdmissionEnabled() && policy.AsNamespacedImageValidatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsNamespacedImageValidatingPolicy().GetStatus().ConditionStatus
	case webhook.ValidatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.ValidatingPolicyType, policy.GetName(), "")
		matchConstraints = policy.AsValidatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsValidatingPolicy().GetSpec().AdmissionEnabled() && policy.AsValidatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsValidatingPolicy().GetStatus().ConditionStatus
	case webhook.NamespacedValidatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.NamespacedValidatingPolicyType, policy.GetName(), policy.GetNamespace())
		matchConstraints = policy.AsNamespacedValidatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsNamespacedValidatingPolicy().GetSpec().AdmissionEnabled() && policy.AsNamespacedValidatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsNamespacedValidatingPolicy().GetStatus().ConditionStatus
	}

	if !backgroundOnly {
		if ready, ok := c.polStateRecorder.Ready(key); ready {
			status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "Webhook configured.")
		} else if ok {
			status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "Policy is not configured in the webhook.")
		}
	}

	gvrs := c.resolveGVRs(matchConstraints.ResourceRules)
	errs := c.permissionsCheck(ctx, gvrs)
	if errs != nil {
		status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionFalse, fmt.Sprintf("Policy is not ready for reporting, missing permissions: %v.", multierr.Combine(errs...)))
	} else {
		status.SetReadyByCondition(policiesv1beta1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionTrue, "Policy is ready for reporting.")
	}

	return status
}

func (c controller) resolveGVRs(rules []admissionregistrationv1.NamedRuleWithOperations) []metav1.GroupVersionResource {
	gvrs := []metav1.GroupVersionResource{}
	for _, rule := range rules {
		for _, g := range rule.RuleWithOperations.APIGroups {
			for _, v := range rule.RuleWithOperations.APIVersions {
				for _, r := range rule.RuleWithOperations.Resources {
					gvrs = append(gvrs, metav1.GroupVersionResource{
						Group:    g,
						Version:  v,
						Resource: r,
					})
				}
			}
		}
	}

	return gvrs
}

func (c controller) permissionsCheck(ctx context.Context, gvrs []metav1.GroupVersionResource) []error {
	var errs []error
	for _, gvr := range gvrs {
		for _, verb := range []string{"get", "list", "watch"} {
			result, err := c.authChecker.Check(ctx, gvr.Group, gvr.Version, gvr.Resource, "", "", "", verb)
			if baseerrors.Is(err, auth.ErrNoServiceAccount) {
				continue
			} else if err != nil {
				errs = append(errs, err)
			} else if !result.Allowed {
				errs = append(errs, fmt.Errorf("%s %s: %s", verb, gvr.String(), result.Reason))
			}
		}
	}

	return errs
}
