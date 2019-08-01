package violation

import (
	"errors"

	"github.com/golang/glog"

	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	lister "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	"k8s.io/apimachinery/pkg/runtime"
)

//Generator to generate policy violation
type Generator interface {
	Add(infos ...*Info) error
	RemoveInactiveViolation(policy, rKind, rNs, rName string, ruleType info.RuleType) error
	ResourceRemoval(policy, rKind, rNs, rName string) error
}

type builder struct {
	client       *client.Client
	policyLister lister.PolicyLister
	eventBuilder event.Generator
}

//Builder is to build policy violations
type Builder interface {
	Generator
	processViolation(info *Info) error
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

//BuldNewViolation returns a new violation
func BuldNewViolation(pName string, rKind string, rNs string, rName string, reason string, frules []v1alpha1.FailedRule) *Info {
	return &Info{
		Policy: pName,
		Violation: v1alpha1.Violation{
			Kind:      rKind,
			Namespace: rNs,
			Name:      rName,
			Reason:    reason,
			Rules:     frules,
		},
	}
}

func (b *builder) Add(infos ...*Info) error {
	if infos == nil {
		return nil
	}
	for _, info := range infos {
		err := b.processViolation(info)
		if err != nil {
			glog.Error(err)
		}
	}
	return nil
}

func (b *builder) processViolation(info *Info) error {
	statusMap := map[string]interface{}{}
	violationsMap := map[string]interface{}{}
	violationMap := map[string]interface{}{}
	var violations interface{}
	var violation interface{}
	// Get Policy
	obj, err := b.client.GetResource("Policy", "", info.Policy, "status")
	if err != nil {
		return err
	}
	unstr := obj.UnstructuredContent()
	// get "status" subresource
	status, ok := unstr["status"]
	if ok {
		// status exists
		// status is already present then we append violations
		if statusMap, ok = status.(map[string]interface{}); !ok {
			return errors.New("Unable to parse status subresource")
		}
		// get policy violations
		violations, ok = statusMap["violations"]
		if !ok {
			return nil
		}
		violationsMap, ok = violations.(map[string]interface{})
		if !ok {
			return errors.New("Unable to get status.violations subresource")
		}
		// check if the resource has a violation
		violation, ok = violationsMap[info.getKey()]
		if !ok {
			// add resource violation
			violationsMap[info.getKey()] = info.Violation
			statusMap["violations"] = violationsMap
			unstr["status"] = statusMap
		} else {
			violationMap, ok = violation.(map[string]interface{})
			if !ok {
				return errors.New("Unable to get status.violations.violation subresource")
			}
			// we check if the new violation updates are different from stored violation info
			v := v1alpha1.Violation{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(violationMap, &v)
			if err != nil {
				return err
			}
			// compare v & info.Violation
			if v.IsEqual(info.Violation) {
				// no updates to violation
				// do nothing
				return nil
			}
			// update the violation
			violationsMap[info.getKey()] = info.Violation
			statusMap["violations"] = violationsMap
			unstr["status"] = statusMap
		}
	} else {
		violationsMap[info.getKey()] = info.Violation
		statusMap["violations"] = violationsMap
		unstr["status"] = statusMap
	}

	obj.SetUnstructuredContent(unstr)
	// update the status sub-resource for policy
	_, err = b.client.UpdateStatusResource("Policy", "", obj, false)
	if err != nil {
		return err
	}
	return nil
}

//RemoveInactiveViolation
func (b *builder) RemoveInactiveViolation(policy, rKind, rNs, rName string, ruleType info.RuleType) error {
	statusMap := map[string]interface{}{}
	violationsMap := map[string]interface{}{}
	violationMap := map[string]interface{}{}
	var violations interface{}
	var violation interface{}
	// Get Policy
	obj, err := b.client.GetResource("Policy", "", policy, "status")
	if err != nil {
		return err
	}
	unstr := obj.UnstructuredContent()
	// get "status" subresource
	status, ok := unstr["status"]
	if !ok {
		return nil
	}
	// status exists
	// status is already present then we append violations
	if statusMap, ok = status.(map[string]interface{}); !ok {
		return errors.New("Unable to parse status subresource")
	}
	// get policy violations
	violations, ok = statusMap["violations"]
	if !ok {
		return nil
	}
	violationsMap, ok = violations.(map[string]interface{})
	if !ok {
		return errors.New("Unable to get status.violations subresource")
	}
	// check if the resource has a violation
	violation, ok = violationsMap[BuildKey(rKind, rNs, rName)]
	if !ok {
		// no violation for this resource
		return nil
	}
	violationMap, ok = violation.(map[string]interface{})
	if !ok {
		return errors.New("Unable to get status.violations.violation subresource")
	}
	// check remove the rules of the given type
	// this is called when the policy is applied succesfully, so we can remove the previous failed rules
	// if all rules are to be removed, the deleted the violation
	v := v1alpha1.Violation{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(violationMap, &v)
	if err != nil {
		return err
	}
	if !v.RemoveRulesOfType(ruleType.String()) {
		// no rule of given type found,
		// no need to remove rule
		return nil
	}
	// if there are no faile rules remove the violation
	if len(v.Rules) == 0 {
		delete(violationsMap, BuildKey(rKind, rNs, rName))
	} else {
		// update the rules
		violationsMap[BuildKey(rKind, rNs, rName)] = v
	}
	statusMap["violations"] = violationsMap
	unstr["status"] = statusMap

	obj.SetUnstructuredContent(unstr)
	// update the status sub-resource for policy
	_, err = b.client.UpdateStatusResource("Policy", "", obj, false)
	if err != nil {
		return err
	}
	return nil
}

// ResourceRemoval on resources reoval we remove the policy violation in the policy
func (b *builder) ResourceRemoval(policy, rKind, rNs, rName string) error {
	statusMap := map[string]interface{}{}
	violationsMap := map[string]interface{}{}
	var violations interface{}
	// Get Policy
	obj, err := b.client.GetResource("Policy", "", policy, "status")
	if err != nil {
		return err
	}
	unstr := obj.UnstructuredContent()
	// get "status" subresource
	status, ok := unstr["status"]
	if !ok {
		return nil
	}
	// status exists
	// status is already present then we append violations
	if statusMap, ok = status.(map[string]interface{}); !ok {
		return errors.New("Unable to parse status subresource")
	}
	// get policy violations
	violations, ok = statusMap["violations"]
	if !ok {
		return nil
	}
	violationsMap, ok = violations.(map[string]interface{})
	if !ok {
		return errors.New("Unable to get status.violations subresource")
	}

	// check if the resource has a violation
	_, ok = violationsMap[BuildKey(rKind, rNs, rName)]
	if !ok {
		// no violation for this resource
		return nil
	}
	// remove the pair from the map
	delete(violationsMap, BuildKey(rKind, rNs, rName))
	if len(violationsMap) == 0 {
		delete(statusMap, "violations")
	} else {
		statusMap["violations"] = violationsMap
	}
	unstr["status"] = statusMap

	obj.SetUnstructuredContent(unstr)
	// update the status sub-resource for policy
	_, err = b.client.UpdateStatusResource("Policy", "", obj, false)
	if err != nil {
		return err
	}
	return nil
}
