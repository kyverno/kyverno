package admissionpolicygenerator

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	policiesv1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1alpha1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	"k8s.io/client-go/kubernetes"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 2
	ControllerName = "admissionpolicy-generator"
	maxRetries     = 10
)

// This controller is responsible for the following:
// - Generating ValidatingAdmissionPolicies and their bindings from Kyverno ClusterPolicies
// - Generating ValidatingAdmissionPolicies and their bindings from Kyverno ValidatingPolicies
// - Generating MutatingAdmissionPolicies and their bindings from Kyverno MutatingPolicies
type controller struct {
	// clients
	client          kubernetes.Interface
	kyvernoClient   versioned.Interface
	discoveryClient dclient.IDiscovery

	// listers
	cpolLister       kyvernov1listers.ClusterPolicyLister
	vpolLister       policiesv1alpha1listers.ValidatingPolicyLister
	mpolLister       policiesv1alpha1listers.MutatingPolicyLister
	polexLister      kyvernov2listers.PolicyExceptionLister
	celpolexLister   policiesv1alpha1listers.PolicyExceptionLister
	vapLister        admissionregistrationv1listers.ValidatingAdmissionPolicyLister
	vapbindingLister admissionregistrationv1listers.ValidatingAdmissionPolicyBindingLister
	mapLister        admissionregistrationv1alpha1listers.MutatingAdmissionPolicyLister
	mapbindingLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyBindingLister

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	eventGen event.Interface
	checker  checker.AuthChecker
}

func NewController(
	client kubernetes.Interface,
	kyvernoClient versioned.Interface,
	discoveryClient dclient.IDiscovery,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vpolInformer policiesv1alpha1informers.ValidatingPolicyInformer,
	mpolInformer policiesv1alpha1informers.MutatingPolicyInformer,
	polexInformer kyvernov2informers.PolicyExceptionInformer,
	celpolexInformer policiesv1alpha1informers.PolicyExceptionInformer,
	vapInformer admissionregistrationv1informers.ValidatingAdmissionPolicyInformer,
	vapbindingInformer admissionregistrationv1informers.ValidatingAdmissionPolicyBindingInformer,
	mapInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyInformer,
	mapbindingInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyBindingInformer,
	eventGen event.Interface,
	checker checker.AuthChecker,
) controllers.Controller {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
	)
	c := &controller{
		client:          client,
		kyvernoClient:   kyvernoClient,
		discoveryClient: discoveryClient,
		cpolLister:      cpolInformer.Lister(),
		vpolLister:      vpolInformer.Lister(),
		mpolLister:      mpolInformer.Lister(),
		polexLister:     polexInformer.Lister(),
		celpolexLister:  celpolexInformer.Lister(),
		queue:           queue,
		eventGen:        eventGen,
		checker:         checker,
	}

	// Set up an event handler for when Kyverno policies change
	if _, err := controllerutils.AddEventHandlersT(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	// Set up an event handler for when validating policies change
	if _, err := controllerutils.AddEventHandlersT(vpolInformer.Informer(), c.addVP, c.updateVP, c.deleteVP); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	// Set up an event handler for when mutating policies change
	if _, err := controllerutils.AddEventHandlersT(mpolInformer.Informer(), c.addMP, c.updateMP, c.deleteMP); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	// Set up an event handler for when policy exceptions change
	if _, err := controllerutils.AddEventHandlersT(polexInformer.Informer(), c.addException, c.updateException, c.deleteException); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	// Set up an event handler for when cel policy exceptions change
	if _, err := controllerutils.AddEventHandlersT(celpolexInformer.Informer(), c.addCELException, c.updateCELException, c.deleteCELException); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	// Set up an event handler for when ValidatingAdmissionPolicies change
	if vapInformer != nil {
		c.vapLister = vapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(vapInformer.Informer(), c.addVAP, c.updateVAP, c.deleteVAP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}

	// Set up an event handler for when ValidatingAdmissionPolicyBindings change
	if vapbindingInformer != nil {
		c.vapbindingLister = vapbindingInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(vapbindingInformer.Informer(), c.addVAPbinding, c.updateVAPbinding, c.deleteVAPbinding); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}

	// Set up an event handler for when MutatingAdmissionPolicies change
	if mapInformer != nil {
		c.mapLister = mapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(mapInformer.Informer(), c.addMAP, c.updateMAP, c.deleteMAP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}

	// Set up an event handler for when MutatingAdmissionPolicyBindings change
	if mapbindingInformer != nil {
		c.mapbindingLister = mapbindingInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(mapbindingInformer.Informer(), c.addMAPbinding, c.updateMAPbinding, c.deleteMAPbinding); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}

	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	var policy engineapi.GenericPolicy

	polType := strings.Split(key, "/")[0]
	if polType == "ClusterPolicy" {
		generateValidatingAdmissionPolicy := toggle.FromContext(context.TODO()).GenerateValidatingAdmissionPolicy()
		if !generateValidatingAdmissionPolicy {
			return nil
		}
		cpol, err := c.getClusterPolicy(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "unable to get the policy from policy informer")
			return err
		}
		spec := cpol.GetSpec()
		if !spec.HasValidate() {
			return nil
		}
		policy = engineapi.NewKyvernoPolicy(cpol)
		err = c.handleVAPGeneration(ctx, polType, policy)
		if err != nil {
			return err
		}
	} else if polType == "ValidatingPolicy" {
		generateValidatingAdmissionPolicy := toggle.FromContext(context.TODO()).GenerateValidatingAdmissionPolicy()
		if !generateValidatingAdmissionPolicy {
			return nil
		}
		vpol, err := c.getValidatingPolicy(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "unable to get the policy from policy informer")
			return err
		}
		policy = engineapi.NewValidatingPolicy(vpol)
		err = c.handleVAPGeneration(ctx, polType, policy)
		if err != nil {
			return err
		}
	} else if polType == "MutatingPolicy" {
		generateMutatingAdmissionPolicy := toggle.FromContext(context.TODO()).GenerateMutatingAdmissionPolicy()
		if !generateMutatingAdmissionPolicy {
			return nil
		}
		mpol, err := c.getMutatingPolicy(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "unable to get the policy from policy informer")
			return err
		}
		err = c.handleMAPGeneration(ctx, mpol)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) updatePolicyStatus(ctx context.Context, policy engineapi.GenericPolicy, generated bool, msg string) {
	if pol := policy.AsKyvernoPolicy(); pol != nil {
		cpol := pol.(*kyvernov1.ClusterPolicy)
		latest := cpol.DeepCopy()
		latest.Status.ValidatingAdmissionPolicy.Generated = generated
		latest.Status.ValidatingAdmissionPolicy.Message = msg

		new, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			logging.Error(err, "failed to update cluster policy status", cpol.GetName(), "status", new.Status)
		}
		logging.V(3).Info("updated cluster policy status", "name", cpol.GetName(), "status", new.Status)
	} else if vpol := policy.AsValidatingPolicy(); vpol != nil {
		latest := vpol.DeepCopy()
		latest.Status.Generated = generated
		latest.Status.GetConditionStatus().Message = msg

		new, err := c.kyvernoClient.PoliciesV1alpha1().ValidatingPolicies().UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			logging.Error(err, "failed to update validating policy status", vpol.GetName(), "status", new.Status)
		}

		logging.V(3).Info("updated validating policy status", "name", vpol.GetName(), "status", new.Status)
	} else if mpol := policy.AsMutatingPolicy(); mpol != nil {
		latest := mpol.DeepCopy()
		latest.Status.Generated = generated
		latest.Status.GetConditionStatus().Message = msg

		new, err := c.kyvernoClient.PoliciesV1alpha1().MutatingPolicies().UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			logging.Error(err, "failed to update mutating policy status", mpol.GetName(), "status", new.Status)
		}

		logging.V(3).Info("updated mutating policy status", "name", mpol.GetName(), "status", new.Status)
	}
}
