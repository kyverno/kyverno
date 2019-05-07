package violation

import (
	"encoding/json"
	"fmt"
	"log"

	jsonpatch "github.com/evanphx/json-patch"
	controllerinterfaces "github.com/nirmata/kube-policy/controller/interfaces"
	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	eventinterfaces "github.com/nirmata/kube-policy/pkg/event/interfaces"
	eventutils "github.com/nirmata/kube-policy/pkg/event/utils"
	violationinterfaces "github.com/nirmata/kube-policy/pkg/violation/interfaces"
	utils "github.com/nirmata/kube-policy/pkg/violation/utils"
	mergetypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type builder struct {
	kubeClient   *kubeClient.KubeClient
	controller   controllerinterfaces.PolicyGetter
	eventBuilder eventinterfaces.BuilderInternal
	logger       *log.Logger
}

type Builder interface {
	violationinterfaces.ViolationGenerator
	ProcessViolation(info utils.ViolationInfo) error
	Patch(policy *types.Policy, updatedPolicy *types.Policy) error
	IsActive(kind string, resource string) (bool, error)
}

func NewViolationBuilder(
	kubeClient *kubeClient.KubeClient,
	eventBuilder eventinterfaces.BuilderInternal,
	logger *log.Logger) (Builder, error) {

	builder := &builder{
		kubeClient:   kubeClient,
		eventBuilder: eventBuilder,
		logger:       logger,
	}
	return builder, nil
}

func (b *builder) Create(info utils.ViolationInfo) error {
	err := b.ProcessViolation(info)
	if err != nil {
		return err
	}
	return nil
}

func (b *builder) SetController(controller controllerinterfaces.PolicyGetter) {
	b.controller = controller
}

func (b *builder) ProcessViolation(info utils.ViolationInfo) error {
	// Get the policy
	policy, err := b.controller.GetPolicy(info.Policy)
	if err != nil {
		utilruntime.HandleError(err)
		return err
	}
	modifiedPolicy := policy.DeepCopy()
	modifiedViolations := []types.Violation{}

	// Create new violation
	newViolation := types.Violation{
		Kind:     info.Kind,
		Resource: info.Resource,
		Rule:     info.Rule,
		Reason:   info.Reason,
		Message:  info.Message,
	}

	for _, violation := range modifiedPolicy.Status.Violations {
		ok, err := b.IsActive(info.Kind, violation.Resource)
		if err != nil {
			utilruntime.HandleError(err)
			continue
		}
		if !ok {
			// Remove the violation
			// Create a removal event
			b.eventBuilder.AddEvent(eventutils.EventInfo{
				Kind:     "Policy",
				Resource: info.Policy,
				Rule:     info.Rule,
				Reason:   info.Reason,
				Message:  info.Message,
			})
			continue
		}
		// If violation already exists for this rule, we update the violation
		//TODO: update violation, instead of re-creating one every time
	}
	modifiedViolations = append(modifiedViolations, newViolation)

	modifiedPolicy.Status.Violations = modifiedViolations
	//	return b.Patch(policy, modifiedPolicy)
	// Violations are part of the status sub resource, so we can use the Update Status api instead of updating the policy object
	return b.controller.UpdatePolicyViolations(modifiedPolicy)
}

func (b *builder) IsActive(kind string, resource string) (bool, error) {
	// Generate Merge Patch
	_, err := b.kubeClient.GetResource(kind, resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get resource %s ", resource))
		return false, err
	}
	return true, nil
}

func (b *builder) Patch(policy *types.Policy, updatedPolicy *types.Policy) error {
	originalData, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	modifiedData, err := json.Marshal(updatedPolicy)
	if err != nil {
		return err
	}
	// generate merge patch
	patchBytes, err := jsonpatch.CreateMergePatch(originalData, modifiedData)
	if err != nil {
		return err
	}
	_, err = b.controller.PatchPolicy(policy.Name, mergetypes.MergePatchType, patchBytes)
	if err != nil {

		// Unable to patch
		return err
	}
	return nil
}
