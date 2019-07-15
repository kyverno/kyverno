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
	currVs := map[string]interface{}{}
	statusMap := map[string]interface{}{}
	var ok bool
	//TODO: hack get from client
	p1, err := b.client.GetResource("Policy", "", info.Policy, "status")
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
		// Violations map[string][]Violations
		glog.Info(reflect.TypeOf(violations))
		if currVs, ok = violations.(map[string]interface{}); !ok {
			return errors.New("Unable to parse violations")
		}
	}
	// Info:
	// Resource - Kind, Namespace, Name
	// policy - Name
	//	violation, ok := currVs[info.getKey()]
	// Key -> resource
	// 1> Check if there were any previous violations for the given key
	// 2> If No, create a new one
	if !ok {
		currVs[info.getKey()] = info.Violation
	} else {
		currV := currVs[info.getKey()]
		glog.Info(reflect.TypeOf(currV))
		v, ok := currV.(map[string]interface{})
		if !ok {
			glog.Info("type not matching")
		}
		// get rules
		rules, ok := v["rules"]
		if !ok {
			glog.Info("rules not found")
		}
		glog.Info(reflect.TypeOf(rules))
		rs, ok := rules.([]interface{})
		if !ok {
			glog.Info("type not matching")
		}
		// check if rules are samre
		if isRuleNamesEqual(rs, info.Violation.Rules) {
			return nil
		}
		// else update the errors
		currVs[info.getKey()] = info.Violation
	}
	// newViolation := info.Violation
	// for _, violation := range currentViolations {
	// 	glog.Info(reflect.TypeOf(violation))
	// 	if v, ok := violation.(map[string]interface{}); ok {
	// 		if name, ok := v["name"].(string); ok {
	// 			if namespace, ok := v["namespace"].(string); ok {
	// 				ok, err := b.isActive(info.Kind, name, namespace)
	// 				if err != nil {
	// 					glog.Error(err)
	// 					continue
	// 				}
	// 				if !ok {
	// 					//TODO remove the violation as it corresponds to resource that does not exist
	// 					glog.Info("removed violation")
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	// currentViolations = append(currentViolations, newViolation)
	// // update violations
	// set the updated status
	statusMap["violations"] = currVs
	unstr["status"] = statusMap
	p1.SetUnstructuredContent(unstr)
	_, err = b.client.UpdateStatusResource("Policy", "", p1, false)
	if err != nil {
		return err
	}
	return nil
}

func (b *builder) isActive(kind, rname, rnamespace string) (bool, error) {
	// Generate Merge Patch
	_, err := b.client.GetResource(b.client.DiscoveryClient.GetGVRFromKind(kind).Resource, rnamespace, rname)
	if err != nil {
		glog.Errorf("unable to get resource %s/%s ", rnamespace, rname)
		return false, err
	}
	return true, nil
}

//NewViolation return new policy violation
func NewViolation(reason event.Reason, policyName, kind, rname, rnamespace, msg string) *Info {
	return &Info{Policy: policyName,
		Violation: types.Violation{
			Kind:      kind,
			Name:      rname,
			Namespace: rnamespace,
			Reason:    reason.String(),
		},
	}
}

// //NewViolationFromEvent returns violation info from event
// func NewViolationFromEvent(e *event.Info, pName, rKind, rName, rnamespace string) *Info {
// 	return &Info{
// 		Policy: pName,
// 		Violation: types.Violation{
// 			Kind:      rKind,
// 			Name:      rName,
// 			Namespace: rnamespace,
// 			Reason:    e.Reason,
// 			Message:   e.Message,
// 		},
// 	}
// }
// Build a new Violation
func BuldNewViolation(pName string, rKind string, rNs string, rName string, reason string, rules []string) *Info {
	return &Info{
		Policy: pName,
		Violation: types.Violation{
			Kind:      rKind,
			Namespace: rNs,
			Name:      rName,
			Reason:    reason,
			Rules:     rules,
		},
	}
}

func isRuleNamesEqual(currRules []interface{}, newRules []string) bool {
	if len(currRules) != len(newRules) {
		return false
	}
	for i, r := range currRules {
		name, ok := r.(string)
		if !ok {
			return false
		}
		if name != newRules[i] {
			return false
		}
	}
	return true
}

//RemoveViolation will remove the violation for the resource if there was one
func RemoveViolation(policy *types.Policy, rKind string, rNs string, rName string) {
	// Remove the <resource, Violation> pair from map
	if policy.Status.Violations != nil {
		glog.Infof("Cleaning up violalation for policy %s, resource %s/%s/%s", policy.Name, rKind, rNs, rName)
		delete(policy.Status.Violations, BuildKey(rKind, rNs, rName))
	}
}
