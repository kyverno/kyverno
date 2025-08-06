package policystatus

import (
	"context"
	baseerrors "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	auth "github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	policiesv1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1alpha1"
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
	vpolInformer policiesv1alpha1informers.ValidatingPolicyInformer,
	ivpolInformer policiesv1alpha1informers.ImageValidatingPolicyInformer,
	mpolInformer policiesv1alpha1informers.MutatingPolicyInformer,
	gpolInformer policiesv1alpha1informers.GeneratingPolicyInformer,
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
			vpol, ok := obj.(*policiesv1alpha1.ValidatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.ValidatingPolicyType, vpol.Name))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for ValidatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		ivpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			ivpol, ok := obj.(*policiesv1alpha1.ImageValidatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.ImageValidatingPolicyType, ivpol.Name))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for ImageValidatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		mpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			mpol, ok := obj.(*policiesv1alpha1.MutatingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.MutatingPolicyType, mpol.Name))
		},
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers for MutatingPolicy")
	}

	_, _, err = controllerutils.AddExplicitEventHandlers(
		logger,
		gpolInformer.Informer(),
		c.queue,
		func(obj interface{}) cache.ExplicitKey {
			gpol, ok := obj.(*policiesv1alpha1.GeneratingPolicy)
			if !ok {
				return ""
			}
			return cache.ExplicitKey(webhook.BuildRecorderKey(webhook.GeneratingPolicyType, gpol.Name))
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

func (c controller) reconcile(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error {
	polType, polName := webhook.ParseRecorderKey(key)
	if polType == webhook.ValidatingPolicyType {
		vpol, err := c.client.PoliciesV1alpha1().ValidatingPolicies().Get(ctx, polName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("validating policy not found", "name", polName)
				return nil
			}
			return err
		}

		return c.updateVpolStatus(ctx, vpol)
	}
	if polType == webhook.ImageValidatingPolicyType {
		ivpol, err := c.client.PoliciesV1alpha1().ImageValidatingPolicies().Get(ctx, polName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("imageVerification policy not found", "name", polName)
				return nil
			}
			return err
		}
		return c.updateIvpolStatus(ctx, ivpol)
	}

	if polType == webhook.MutatingPolicyType {
		mpol, err := c.client.PoliciesV1alpha1().MutatingPolicies().Get(ctx, polName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("mutating policy not found", "name", polName)
				return nil
			}
		}
		return c.updateMpolStatus(ctx, mpol)
	}

	if polType == webhook.GeneratingPolicyType {
		gpol, err := c.client.PoliciesV1alpha1().GeneratingPolicies().Get(ctx, polName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("generating policy not found", "name", polName)
				return nil
			}
			return err
		}
		return c.updateGpolStatus(ctx, gpol)
	}
	return nil
}

func (c controller) reconcileConditions(ctx context.Context, policy engineapi.GenericPolicy) *policiesv1alpha1.ConditionStatus {
	var key string
	var matchConstraints admissionregistrationv1.MatchResources
	status := &policiesv1alpha1.ConditionStatus{}
	backgroundOnly := false
	switch policy.GetKind() {
	case webhook.ValidatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.ValidatingPolicyType, policy.GetName())
		matchConstraints = policy.AsValidatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsValidatingPolicy().GetSpec().AdmissionEnabled() && policy.AsValidatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsValidatingPolicy().GetStatus().ConditionStatus
	case webhook.ImageValidatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.ImageValidatingPolicyType, policy.GetName())
		matchConstraints = policy.AsImageValidatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsImageValidatingPolicy().GetSpec().AdmissionEnabled() && policy.AsImageValidatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsImageValidatingPolicy().GetStatus().ConditionStatus
	case webhook.MutatingPolicyType:
		key = webhook.BuildRecorderKey(webhook.MutatingPolicyType, policy.GetName())
		matchConstraints = policy.AsMutatingPolicy().GetMatchConstraints()
		backgroundOnly = (!policy.AsMutatingPolicy().GetSpec().AdmissionEnabled() && policy.AsMutatingPolicy().GetSpec().BackgroundEnabled())
		status = &policy.AsMutatingPolicy().GetStatus().ConditionStatus
	case webhook.GeneratingPolicyType:
		key = webhook.BuildRecorderKey(webhook.GeneratingPolicyType, policy.GetName())
		matchConstraints = policy.AsGeneratingPolicy().GetMatchConstraints()
		status = &policy.AsGeneratingPolicy().GetStatus().ConditionStatus
	}

	if !backgroundOnly {
		if ready, ok := c.polStateRecorder.Ready(key); ready {
			status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "Webhook configured.")
		} else if ok {
			status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "Policy is not configured in the webhook.")
		}
	}

	gvrs := []metav1.GroupVersionResource{}
	for _, rule := range matchConstraints.ResourceRules {
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

	var errs []error
	for _, gvr := range gvrs {
		for _, verb := range []string{"get", "list", "watch"} {
			result, err := c.authChecker.Check(ctx, gvr.Group, gvr.Version, gvr.Resource, "", "", "", verb)
			if baseerrors.Is(err, checker.ErrNoServiceAccount) {
				continue
			} else if err != nil {
				errs = append(errs, err)
			} else if !result.Allowed {
				errs = append(errs, fmt.Errorf("%s %s: %s", verb, gvr.String(), result.Reason))
			}
		}
	}

	if errs != nil {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionFalse, fmt.Sprintf("Policy is not ready for reporting, missing permissions: %v.", multierr.Combine(errs...)))
	} else {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionTrue, "Policy is ready for reporting.")
	}
	return status
}
