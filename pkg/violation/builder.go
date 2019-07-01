package violation

import (
	"errors"
	"reflect"

	"github.com/golang/glog"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	v1alpha1 "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
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
	isActive(kind string, rname string, rnamespace string) (bool, error)
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
	currentViolations := []interface{}{}
	statusMap := map[string]interface{}{}
	var ok bool
	//TODO: hack get from client
	p1, err := b.client.GetResource("policies", "", info.Policy, "status")
	if err != nil {
		return err
	}
	unstr := p1.UnstructuredContent()
	// check if "status" field exists
	status, ok := unstr["status"]
	if ok {
		// status is already present then we append violations
		if statusMap, ok = status.(map[string]interface{}); !ok {
			return errors.New("Unable to parse status subresource")
		}
		violations, ok := statusMap["violations"]
		if !ok {
			glog.Info("violation not present")
		}
		glog.Info(reflect.TypeOf(violations))
		if currentViolations, ok = violations.([]interface{}); !ok {
			return errors.New("Unable to parse violations")
		}
	}
	newViolation := info.Violation
	for _, violation := range currentViolations {
		glog.Info(reflect.TypeOf(violation))
		if v, ok := violation.(map[string]interface{}); ok {
			if name, ok := v["name"].(string); ok {
				if namespace, ok := v["namespace"].(string); ok {
					ok, err := b.isActive(info.Kind, name, namespace)
					if err != nil {
						glog.Error(err)
						continue
					}
					if !ok {
						//TODO remove the violation as it corresponds to resource that does not exist
						glog.Info("removed violation")
					}
				}
			}
		}
	}
	currentViolations = append(currentViolations, newViolation)
	// update violations
	// set the updated status
	statusMap["violations"] = currentViolations
	unstr["status"] = statusMap
	p1.SetUnstructuredContent(unstr)
	_, err = b.client.UpdateStatusResource("policies", "", p1, false)
	if err != nil {
		return err
	}
	return nil
	// modifiedViolations := []types.Violation{}
	// modifiedViolations = append(modifiedViolations, types.Violation{Name: "name", Kind: "Deploymeny"})
	// unstr["status"] = modifiedViolations
	// p1.SetUnstructuredContent(unstr)
	// rdata, err := p1.MarshalJSON()
	// if err != nil {
	// 	glog.Info(err)
	// }
	// glog.Info(string(rdata))
	// _, err = b.client.UpdateStatusResource("policies", "", p1, false)
	// if err != nil {
	// 	glog.Info(err)
	// }

	// p, err := b.policyLister.Get(info.Policy)
	// if err != nil {
	// 	glog.Error(err)
	// 	return err
	// }

	// glog.Info(p.TypeMeta.Kind)
	// glog.Info(p.Kind)
	// modifiedPolicy := p.DeepCopy()
	// glog.Info(modifiedPolicy.Kind)
	// // Create new violation
	// newViolation := info.Violation

	// for _, violation := range modifiedPolicy.Status.Violations {
	// 	ok, err := b.isActive(info.Kind, violation.Name)
	// 	if err != nil {
	// 		glog.Error(err)
	// 		continue
	// 	}
	// 	if !ok {
	// 		glog.Info("removed violation")
	// 	}
	// }
	// // If violation already exists for this rule, we update the violation
	// //TODO: update violation, instead of re-creating one every time
	// modifiedViolations = append(modifiedViolations, newViolation)
	// modifiedPolicy.Status.Violations = modifiedViolations
	// // Violations are part of the status sub resource, so we can use the Update Status api instead of updating the policy object
	// _, err = b.client.UpdateStatusResource("policies", "", *modifiedPolicy, false)
	// if err != nil {
	// 	glog.Info(err)
	// 	return err
	// }
	// return nil
}

func (b *builder) isActive(kind string, rname string, rnamespace string) (bool, error) {
	// Generate Merge Patch
	_, err := b.client.GetResource(b.client.DiscoveryClient.GetGVRFromKind(kind).Resource, rnamespace, rname)
	if err != nil {
		glog.Errorf("unable to get resource %s/%s ", rnamespace, rname)
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
			Reason:    e.Reason,
			Message:   e.Message,
		},
	}
}
