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
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"go.uber.org/multierr"
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
	dclient     dclient.Interface
	client      versioned.Interface
	queue       workqueue.TypedRateLimitingInterface[any]
	authChecker auth.AuthChecker
}

func NewController(dclient dclient.Interface, client versioned.Interface, vpolInformer policiesv1alpha1informers.ValidatingPolicyInformer, reportsSA string) Controller {
	c := &controller{
		dclient: dclient,
		client:  client,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName}),
		authChecker: auth.NewSubjectChecker(dclient.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), reportsSA, nil),
	}

	enqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(controllerutils.MetaNamespaceKey, controllerutils.Queue(c.queue)))
	_, err := controllerutils.AddEventHandlers(
		vpolInformer.Informer(),
		controllerutils.AddFunc(logger, enqueueFunc),
		func(old, new interface{}) {
			oldVpol := old.(*policiesv1alpha1.ValidatingPolicy)
			newVpol := new.(*policiesv1alpha1.ValidatingPolicy)
			if !datautils.DeepEqual(oldVpol.GetStatus(), newVpol.GetStatus()) {
				if err := enqueueFunc(new); err != nil {
					logger.Error(err, "failed to enqueue object", "obj", new)
				}
			}
		},
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return c
}

func (c controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c controller) reconcile(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error {
	vpol, err := c.client.PoliciesV1alpha1().ValidatingPolicies().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(4).Info("validating policy not found", "name", name)
			return nil
		}
		return err
	}

	c.reconcileConditions(ctx, vpol)
	return c.setReady(ctx, vpol)
}

func (c controller) reconcileConditions(ctx context.Context, vpol *policiesv1alpha1.ValidatingPolicy) {
	gvrs := []metav1.GroupVersionResource{}
	for _, rule := range vpol.GetMatchConstraints().ResourceRules {
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
		vpol.GetStatus().SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionFalse, fmt.Sprintf("missing permissions: %v", multierr.Combine(errs...)))
	} else {
		vpol.GetStatus().SetReadyByCondition(policiesv1alpha1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionTrue, "Policy is ready for reporting")
	}
}

func (c controller) setReady(ctx context.Context, vpol *policiesv1alpha1.ValidatingPolicy) error {
	status := vpol.GetStatus()
	ready := true
	for _, condition := range status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			ready = false
			break
		}
	}

	updateFunc := func(vpol *policiesv1alpha1.ValidatingPolicy) error {
		status := vpol.GetStatus()
		if status.Ready == nil || *status.Ready != ready {
			status.Ready = &ready
		}
		return nil
	}

	err := controllerutils.UpdateStatus(ctx,
		vpol,
		c.client.PoliciesV1alpha1().ValidatingPolicies(),
		updateFunc,
		func(current, expect *policiesv1alpha1.ValidatingPolicy) bool {
			if current.GetStatus().Ready == nil || current.GetStatus().Ready == expect.GetStatus().Ready {
				return false
			}

			for _, condition := range current.GetStatus().Conditions {
				for _, expectCondition := range expect.GetStatus().Conditions {
					if condition.Type == expectCondition.Type && condition.Status == expectCondition.Status {
						return false
					}
				}
			}
			return true
		},
	)
	return err
}
