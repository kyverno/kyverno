package mutate

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineUtils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cache "k8s.io/client-go/tools/cache"
)

type MutateExistingController struct {
	client *dclient.Client

	// typed client for Kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset

	// urStatusControl is used to update UR status
	statusControl common.StatusControlInterface

	// event generator interface
	eventGen event.Interface

	log logr.Logger

	// urLister can list/get update request from the shared informer's store
	urLister urlister.UpdateRequestNamespaceLister

	// policyLister can list/get cluster policy from the shared informer's store
	policyLister kyvernolister.ClusterPolicyLister

	// policyLister can list/get Namespace policy from the shared informer's store
	npolicyLister kyvernolister.PolicyLister

	Config config.Interface
}

// NewMutateExistingController returns an instance of the MutateExistingController
func NewMutateExistingController(
	kyvernoClient *kyvernoclient.Clientset,
	client *dclient.Client,
	policyLister kyvernolister.ClusterPolicyLister,
	npolicyLister kyvernolister.PolicyLister,
	urLister urlister.UpdateRequestNamespaceLister,
	eventGen event.Interface,
	log logr.Logger,
	dynamicConfig config.Interface,
) (*MutateExistingController, error) {

	c := MutateExistingController{
		client:        client,
		kyvernoClient: kyvernoClient,
		eventGen:      eventGen,
		log:           log,
		policyLister:  policyLister,
		npolicyLister: npolicyLister,
		urLister:      urLister,
		Config:        dynamicConfig,
	}

	c.statusControl = common.StatusControl{Client: kyvernoClient}
	return &c, nil
}

func (c *MutateExistingController) ProcessUR(ur *urkyverno.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.Name, "policy", ur.Spec.Policy, "kind", ur.Spec.Resource.Kind, "apiVersion", ur.Spec.Resource.APIVersion, "namespace", ur.Spec.Resource.Namespace, "name", ur.Spec.Resource.Name)
	var errs []error

	policy, err := c.getPolicy(ur.Spec.Policy)
	if err != nil {
		logger.Error(err, "failed to get policy")
		return err
	}

	for _, rule := range policy.GetSpec().Rules {
		if !rule.IsMutateExisting() {
			continue
		}

		trigger, err := common.GetResource(c.client, ur.Spec, c.log)
		if err != nil {
			logger.WithName(rule.Name).Error(err, "failed to get trigger resource")
			errs = append(errs, err)
		}

		policyContext, _, err := common.NewBackgroundContext(c.client, ur, policy, trigger, nil, c.Config, nil)
		if err != nil {
			logger.WithName(rule.Name).Error(err, "failed to build policy context")
			errs = append(errs, err)
		}

		er := engine.Mutate(policyContext)
		for _, r := range er.PolicyResponse.Rules {
			patched := r.PatchedTarget
			switch r.Status {
			case response.RuleStatusFail, response.RuleStatusError, response.RuleStatusWarn:
				err := fmt.Errorf("failed to mutate existing resource, rule response%v: %s", r.Status, r.Message)
				logger.Error(err, "")
				errs = append(errs, err)
				c.report(err, ur.Spec.Policy, rule.Name, patched)

			case response.RuleStatusSkip:
				logger.Info("mutate existing rule skipped", "rule", r.Name, "message", r.Message)
				c.report(err, ur.Spec.Policy, rule.Name, patched)

			case response.RuleStatusPass:
				if r.Status == response.RuleStatusPass {
					_, updateErr := c.client.UpdateResource(patched.GetAPIVersion(), patched.GetKind(), patched.GetNamespace(), patched.Object, false)
					if updateErr != nil {
						errs = append(errs, updateErr)
						logger.WithName(rule.Name).Error(updateErr, "failed to update target resource", "namespace", patched.GetNamespace(), "name", patched.GetName())
					} else {
						logger.WithName(rule.Name).V(4).Info("successfully mutated existing resource", "namespace", patched.GetNamespace(), "name", patched.GetName())
					}

					c.report(updateErr, ur.Spec.Policy, rule.Name, patched)
				}
			}
		}
	}

	return updateURStatus(c.statusControl, *ur, engineUtils.CombineErrors(errs))
}

func (c *MutateExistingController) getPolicy(key string) (kyvernov1.PolicyInterface, error) {
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if pNamespace != "" {
		return c.npolicyLister.Policies(pNamespace).Get(pName)
	}

	return c.policyLister.Get(pName)
}

func (c *MutateExistingController) report(err error, policy, rule string, target *unstructured.Unstructured) {
	var events []event.Info

	if target == nil {
		c.log.WithName("mutateExisting").Info("cannot generate events for empty target resource", "policy", policy, "rule", rule, "err", err.Error())
	}

	if err != nil {
		events = common.FailedEvents(err, policy, rule, event.MutateExistingController, *target)
	} else {
		events = common.SucceedEvents(policy, rule, event.MutateExistingController, *target)
	}

	c.eventGen.Add(events...)
}

func updateURStatus(statusControl common.StatusControlInterface, ur urkyverno.UpdateRequest, err error) error {
	if err != nil {
		return statusControl.Failed(ur, err.Error(), nil)
	}

	return statusControl.Success(ur, nil)
}
