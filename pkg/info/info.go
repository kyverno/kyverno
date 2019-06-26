package info

import (
	"fmt"
	"strings"
)

//PolicyInfo defines policy information
type PolicyInfo struct {
	Name      string
	Resource  string
	Namespace string
	success   bool
	rules     []*RuleInfo
}

//NewPolicyInfo returns a new policy info
func NewPolicyInfo(policyName string, resource string, ns string) *PolicyInfo {
	return &PolicyInfo{
		Name:      policyName,
		Resource:  resource,
		Namespace: ns,
		success:   true, // fail to be set explicity
	}
}

//IsSuccessful checks if policy is succesful
// the policy is set to fail, if any of the rules have failed
func (pi *PolicyInfo) IsSuccessful() bool {
	return pi.success
}

//ErrorRules returns error msgs from all rule
func (pi *PolicyInfo) ErrorRules() string {
	errorMsgs := []string{}
	for _, r := range pi.rules {
		if !r.IsSuccessful() {
			errorMsgs = append(errorMsgs, r.ToString())
		}
	}
	return strings.Join(errorMsgs, ";")
}

//RuleInfo defines rule struct
type RuleInfo struct {
	Name    string
	Msgs    []string
	success bool
}

//ToString reule information
func (ri *RuleInfo) ToString() string {
	str := "rulename: " + ri.Name
	msgs := strings.Join(ri.Msgs, ";")
	return strings.Join([]string{str, msgs}, ";")
}

//NewRuleInfo creates a new RuleInfo
func NewRuleInfo(ruleName string) *RuleInfo {
	return &RuleInfo{
		Name:    ruleName,
		Msgs:    []string{},
		success: true, // fail to be set explicity
	}
}

//Fail set the rule as failed
func (ri *RuleInfo) Fail() {
	ri.success = false
}

//IsSuccessful checks if rule is succesful
func (ri *RuleInfo) IsSuccessful() bool {
	return ri.success
}

//Add add msg
func (ri *RuleInfo) Add(msg string) {
	ri.Msgs = append(ri.Msgs, msg)
}

//Addf add msg with args
func (ri *RuleInfo) Addf(msg string, args ...interface{}) {
	ri.Msgs = append(ri.Msgs, fmt.Sprintf(msg, args...))
}

//RulesSuccesfuly check if the any rule has failed or not
func RulesSuccesfuly(rules []*RuleInfo) bool {
	for _, r := range rules {
		if !r.success {
			return false
		}
	}
	return true
}

//AddRuleInfos sets the rule information
func (pi *PolicyInfo) AddRuleInfos(rules []*RuleInfo) {
	if !RulesSuccesfuly(rules) {
		pi.success = false
	}
	pi.rules = rules
}
