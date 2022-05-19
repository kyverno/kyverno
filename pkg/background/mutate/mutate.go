package mutate

import (
	"encoding/json"
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
	"github.com/kyverno/kyverno/pkg/utils"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cache "k8s.io/client-go/tools/cache"
)

var ErrEmptyPatch error = fmt.Errorf("empty resource to patch")

type MutateExistingController struct {
	client dclient.Interface

	// typed client for Kyverno CRDs
	kyvernoClient kyvernoclient.Interface

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

	Config config.Configuration
}

// NewMutateExistingController returns an instance of the MutateExistingController
func NewMutateExistingController(
	kyvernoClient kyvernoclient.Interface,
	client dclient.Interface,
	policyLister kyvernolister.ClusterPolicyLister,
	npolicyLister kyvernolister.PolicyLister,
	urLister urlister.UpdateRequestNamespaceLister,
	eventGen event.Interface,
	log logr.Logger,
	dynamicConfig config.Configuration,
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

	c.statusControl = common.NewStatusControl(kyvernoClient, urLister)
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
			continue
		}

		policyContext, _, err := common.NewBackgroundContext(c.client, ur, policy, trigger, c.Config, nil, logger)
		if err != nil {
			logger.WithName(rule.Name).Error(err, "failed to build policy context")
			errs = append(errs, err)
			continue
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

				patchedNew, err := addAnnotation(policy, patched, r)
				if err != nil {
					logger.Error(err, "failed to apply patches")
					errs = append(errs, err)
				}

				if patchedNew == nil {
					logger.Error(ErrEmptyPatch, "", "rule", r.Name, "message", r.Message)
					errs = append(errs, err)
					continue
				}

				if r.Status == response.RuleStatusPass {
					_, updateErr := c.client.UpdateResource(patchedNew.GetAPIVersion(), patchedNew.GetKind(), patchedNew.GetNamespace(), patchedNew.Object, false)
					if updateErr != nil {
						errs = append(errs, updateErr)
						logger.WithName(rule.Name).Error(updateErr, "failed to update target resource", "namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
					} else {
						logger.WithName(rule.Name).V(4).Info("successfully mutated existing resource", "namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
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
		events = event.NewBackgroundFailedEvent(err, policy, rule, event.MutateExistingController, target)
	} else {
		events = event.NewBackgroundSuccessEvent(policy, rule, event.MutateExistingController, target)
	}

	c.eventGen.Add(events...)
}

func updateURStatus(statusControl common.StatusControlInterface, ur urkyverno.UpdateRequest, err error) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), nil); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), nil); err != nil {
			return err
		}
	}
	return nil
}

func addAnnotation(policy kyvernov1.PolicyInterface, patched *unstructured.Unstructured, r response.RuleResponse) (patchedNew *unstructured.Unstructured, err error) {
	if patched == nil {
		return
	}

	patchedNew = patched
	var rulePatches []utils.RulePatch

	for _, patch := range r.Patches {
		var patchmap map[string]interface{}
		if err := json.Unmarshal(patch, &patchmap); err != nil {
			return nil, fmt.Errorf("failed to parse JSON patch bytes: %v", err)
		}

		rp := struct {
			RuleName string `json:"rulename"`
			Op       string `json:"op"`
			Path     string `json:"path"`
		}{
			RuleName: r.Name,
			Op:       patchmap["op"].(string),
			Path:     patchmap["path"].(string),
		}

		rulePatches = append(rulePatches, rp)
	}

	var annotationContent = make(map[string]string)
	policyName := policy.GetName()
	if policy.GetNamespace() != "" {
		policyName = policy.GetNamespace() + "/" + policy.GetName()
	}

	for _, rulePatch := range rulePatches {
		annotationContent[rulePatch.RuleName+"."+policyName+".kyverno.io"] = utils.OperationToPastTense[rulePatch.Op] + " " + rulePatch.Path
	}

	if len(annotationContent) == 0 {
		return
	}

	result, _ := yamlv2.Marshal(annotationContent)

	ann := patchedNew.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	ann[utils.PolicyAnnotation] = string(result)
	patchedNew.SetAnnotations(ann)

	return
}
