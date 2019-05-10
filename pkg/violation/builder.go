package violation

import (
	"fmt"
	"log"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	policyclientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	policylister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	event "github.com/nirmata/kube-policy/pkg/event"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

type PolicyViolationGenerator interface {
	Add(info ViolationInfo) error
}

type policyViolationBuilder struct {
	kubeClient      *kubeClient.KubeClient
	policyLister    policylister.PolicyLister
	policyInterface policyclientset.Interface
	eventBuilder    event.EventGenerator
	logger          *log.Logger
}

type PolicyViolationBuilder interface {
	PolicyViolationGenerator
	processViolation(info ViolationInfo) error
	isActive(kind string, resource string) (bool, error)
}

func NewPolicyViolationBuilder(
	kubeClient *kubeClient.KubeClient,
	policyLister policylister.PolicyLister,
	policyInterface policyclientset.Interface,
	eventController event.EventGenerator,
	logger *log.Logger) PolicyViolationBuilder {

	builder := &policyViolationBuilder{
		kubeClient:      kubeClient,
		policyLister:    policyLister,
		policyInterface: policyInterface,
		eventBuilder:    eventController,
		logger:          logger,
	}
	return builder
}

func (pvb *policyViolationBuilder) Add(info ViolationInfo) error {
	return pvb.processViolation(info)
}

func (pvb *policyViolationBuilder) processViolation(info ViolationInfo) error {
	// Get the policy
	namespace, name, err := cache.SplitMetaNamespaceKey(info.Policy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to extract namespace and name for %s", info.Policy))
		return err
	}
	policy, err := pvb.policyLister.Policies(namespace).Get(name)
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
		ok, err := pvb.isActive(info.Kind, violation.Resource)
		if err != nil {
			utilruntime.HandleError(err)
			continue
		}
		if !ok {
			pvb.logger.Printf("removed violation ")
		}
	}
	// If violation already exists for this rule, we update the violation
	//TODO: update violation, instead of re-creating one every time
	modifiedViolations = append(modifiedViolations, newViolation)

	modifiedPolicy.Status.Violations = modifiedViolations
	// Violations are part of the status sub resource, so we can use the Update Status api instead of updating the policy object
	_, err = pvb.policyInterface.NirmataV1alpha1().Policies(namespace).UpdateStatus(modifiedPolicy)
	if err != nil {
		return err
	}
	return nil
}

func (pvb *policyViolationBuilder) isActive(kind string, resource string) (bool, error) {
	// Generate Merge Patch
	_, err := pvb.kubeClient.GetResource(kind, resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get resource %s ", resource))
		return false, err
	}
	return true, nil
}
