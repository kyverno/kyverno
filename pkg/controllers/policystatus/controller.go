package policystatus

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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
	dclient           dclient.Interface
	client            versioned.Interface
	queue             workqueue.TypedRateLimitingInterface[any]
	authChecker       auth.AuthChecker
	vpolStateRecorder webhook.StateRecorder
}

func NewController(dclient dclient.Interface, client versioned.Interface, vpolInformer policiesv1alpha1informers.ValidatingPolicyInformer, reportsSA string, vpolStateRecorder webhook.StateRecorder) Controller {
	c := &controller{
		dclient: dclient,
		client:  client,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName}),
		authChecker:       auth.NewSubjectChecker(dclient.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), reportsSA, nil),
		vpolStateRecorder: vpolStateRecorder,
	}

	enqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(controllerutils.MetaNamespaceKey, controllerutils.Queue(c.queue)))
	_, err := controllerutils.AddEventHandlers(
		vpolInformer.Informer(),
		controllerutils.AddFunc(logger, enqueueFunc),
		controllerutils.UpdateFunc(logger, enqueueFunc),
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return c
}

func (c controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.watchdog)
}

func (c *controller) watchdog(ctx context.Context, logger logr.Logger) {
	notifyChan := c.vpolStateRecorder.(*webhook.Recorder).NotifyChan
	for key := range notifyChan {
		c.queue.Add(key)
	}
}

func (c controller) reconcile(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error {
	polType, polName := webhook.ParsePolicyKey(key)
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
	if polType == webhook.ImageVerificationPolicy {
		ivpol, err := c.client.PoliciesV1alpha1().ImageVerificationPolicies().Get(ctx, polName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(4).Info("imageVerification policy not found", "name", polName)
				return nil
			}
			return err
		}
		return c.updateIvpolStatus(ctx, ivpol)
	}
	return nil
}

func (c controller) reconcileConditions(ctx context.Context, policy engineapi.GenericPolicy) {
	var key string
	var matchConstraints admissionregistrationv1.MatchResources
	status := &policiesv1alpha1.ConditionStatus{}
	switch policy.GetKind() {
	case webhook.ValidatingPolicyType:
		key = webhook.BuildPolicyKey(webhook.ValidatingPolicyType, policy.GetName())
		matchConstraints = policy.AsValidatingPolicy().GetMatchConstraints()
	case webhook.ImageVerificationPolicy:
		key = webhook.BuildPolicyKey(webhook.ImageVerificationPolicy, policy.GetName())
		matchConstraints = policy.AsImageVerificationPolicy().GetMatchConstraints()
	}

	if ready, ok := c.vpolStateRecorder.Ready(key); ready {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "Webhook configured.")
	} else if ok {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "Policy is not configured in the webhook.")
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
			if err != nil {
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
}
