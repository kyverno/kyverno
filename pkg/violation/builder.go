package violation

import (
	"fmt"
	"log"
	"os"

	client "github.com/nirmata/kube-policy/client"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	v1alpha1 "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	event "github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/sharedinformer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

//Generator to generate policy violation
type Generator interface {
	Add(info Info) error
}

type builder struct {
	client       *client.Client
	policyLister v1alpha1.PolicyLister
	eventBuilder event.Generator
	logger       *log.Logger
}

//Builder is to build policy violations
type Builder interface {
	Generator
	processViolation(info Info) error
	isActive(kind string, resource string) (bool, error)
}

//NewPolicyViolationBuilder returns new violation builder
func NewPolicyViolationBuilder(client *client.Client,
	sharedInfomer sharedinformer.PolicyInformer,
	eventController event.Generator,
	logger *log.Logger) Builder {

	if logger == nil {
		logger = log.New(os.Stdout, "Violation Builder: ", log.LstdFlags)
	}

	builder := &builder{
		client:       client,
		policyLister: sharedInfomer.GetLister(),
		eventBuilder: eventController,
		logger:       logger,
	}
	return builder
}

func (b *builder) Add(info Info) error {
	return b.processViolation(info)
}

func (b *builder) processViolation(info Info) error {
	// Get the policy
	namespace, name, err := cache.SplitMetaNamespaceKey(info.Policy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to extract namespace and name for %s", info.Policy))
		return err
	}
	policy, err := b.policyLister.Get(name)
	if err != nil {
		utilruntime.HandleError(err)
		return err
	}
	modifiedPolicy := policy.DeepCopy()
	modifiedViolations := []types.Violation{}

	// Create new violation
	newViolation := info.Violation

	for _, violation := range modifiedPolicy.Status.Violations {
		ok, err := b.isActive(info.Kind, violation.Resource)
		if err != nil {
			utilruntime.HandleError(err)
			continue
		}
		if !ok {
			b.logger.Printf("removed violation")
		}
	}
	// If violation already exists for this rule, we update the violation
	//TODO: update violation, instead of re-creating one every time
	modifiedViolations = append(modifiedViolations, newViolation)

	modifiedPolicy.Status.Violations = modifiedViolations
	// Violations are part of the status sub resource, so we can use the Update Status api instead of updating the policy object
	_, err = b.client.UpdateStatusResource("policies", namespace, modifiedPolicy)
	if err != nil {
		return err
	}
	return nil
}

func (b *builder) isActive(kind string, resource string) (bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", resource))
		return false, err
	}
	// Generate Merge Patch
	_, err = b.client.GetResource(kind, namespace, name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get resource %s ", resource))
		return false, err
	}
	return true, nil
}

//NewViolation return new policy violation
func NewViolation(policyName string, kind string, resource string, rule string) Info {
	return Info{Policy: policyName,
		Violation: types.Violation{
			Kind: kind, Resource: resource, Rule: rule},
	}
}
