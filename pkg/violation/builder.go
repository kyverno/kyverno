package violation

import (
	"github.com/golang/glog"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	v1alpha1 "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	"k8s.io/client-go/tools/cache"
)

//Generator to generate policy violation
type Generator interface {
	Add(infos ...*Info) error
}

type builder struct {
	client       *client.Client
	policyLister v1alpha1.PolicyLister
	eventBuilder event.Generator
}

//Builder is to build policy violations
type Builder interface {
	Generator
	processViolation(info *Info) error
	isActive(kind string, resource string) (bool, error)
}

//NewPolicyViolationBuilder returns new violation builder
func NewPolicyViolationBuilder(client *client.Client,
	sharedInfomer sharedinformer.PolicyInformer,
	eventController event.Generator) Builder {

	builder := &builder{
		client:       client,
		policyLister: sharedInfomer.GetLister(),
		eventBuilder: eventController,
	}
	return builder
}

func (b *builder) Add(infos ...*Info) error {
	for _, info := range infos {
		return b.processViolation(info)
	}
	return nil
}

func (b *builder) processViolation(info *Info) error {
	policy, err := b.policyLister.Get(info.Policy)
	if err != nil {
		glog.Error(err)
		return err
	}
	modifiedPolicy := policy.DeepCopy()
	modifiedViolations := []types.Violation{}

	// Create new violation
	newViolation := info.Violation

	for _, violation := range modifiedPolicy.Status.Violations {
		ok, err := b.isActive(info.Kind, violation.Name)
		if err != nil {
			glog.Error(err)
			continue
		}
		if !ok {
			glog.Info("removed violation")
		}
	}
	// If violation already exists for this rule, we update the violation
	//TODO: update violation, instead of re-creating one every time
	modifiedViolations = append(modifiedViolations, newViolation)

	modifiedPolicy.Status.Violations = modifiedViolations
	// Violations are part of the status sub resource, so we can use the Update Status api instead of updating the policy object
	_, err = b.client.UpdateStatusResource("policies", "", modifiedPolicy, false)
	if err != nil {
		return err
	}
	return nil
}

func (b *builder) isActive(kind string, resource string) (bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(resource)
	if err != nil {
		glog.Errorf("invalid resource key: %s", resource)
		return false, err
	}
	// Generate Merge Patch
	_, err = b.client.GetResource(kind, namespace, name)
	if err != nil {
		glog.Errorf("unable to get resource %s ", resource)
		return false, err
	}
	return true, nil
}

//NewViolation return new policy violation
func NewViolation(policyName string, kind string, rname string, rnamespace string, reason string, msg string) Info {
	return Info{Policy: policyName,
		Violation: types.Violation{
			Kind:      kind,
			Name:      rname,
			Namespace: rnamespace,
			Reason:    reason,
		},
	}
}

//NewViolationFromEvent returns violation info from event
func NewViolationFromEvent(e *event.Info, pName string, rKind string, rName string, rnamespace string) *Info {
	return &Info{
		Policy: pName,
		Violation: types.Violation{
			Kind:      rKind,
			Name:      rName,
			Namespace: rnamespace,
			Reason:    e.Message,
		},
	}
}
