package info

import (
	"fmt"
	"strings"

	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
)

//PolicyInfo defines policy information
type PolicyInfo struct {
	// Name is policy name
	Name string
	// RKind represents the resource kind
	RKind string
	// RName is resource name
	RName string
	// Namespace is the ns of resource
	// empty on non-namespaced resources
	RNamespace string
	//TODO: add check/enum for types
	ValidationFailureAction string // BlockChanges, ReportViolation
	Rules                   []*RuleInfo
	success                 bool
}

//NewPolicyInfo returns a new policy info
func NewPolicyInfo(policyName, rKind, rName, rNamespace, validationFailureAction string) *PolicyInfo {
	return &PolicyInfo{
		Name:                    policyName,
		RKind:                   rKind,
		RName:                   rName,
		RNamespace:              rNamespace,
		success:                 true, // fail to be set explicity
		ValidationFailureAction: validationFailureAction,
	}
}

//IsSuccessful checks if policy is succesful
// the policy is set to fail, if any of the rules have failed
func (pi *PolicyInfo) IsSuccessful() bool {
	for _, r := range pi.Rules {
		if !r.success {
			pi.success = false
			return false
		}
	}
	pi.success = true
	return true
}

// SuccessfulRules returns list of successful rule names
func (pi *PolicyInfo) SuccessfulRules() []string {
	var rules []string
	for _, r := range pi.Rules {
		if r.IsSuccessful() {
			rules = append(rules, r.Name)
		}
	}
	return rules
}

// FailedRules returns list of failed rule names
func (pi *PolicyInfo) FailedRules() []string {
	var rules []string
	for _, r := range pi.Rules {
		if !r.IsSuccessful() {
			rules = append(rules, r.Name)
		}
	}
	return rules
}

//GetFailedRules returns the failed rules with rule type
func (pi *PolicyInfo) GetFailedRules() []v1alpha1.FailedRule {
	var rules []v1alpha1.FailedRule
	for _, r := range pi.Rules {
		if !r.IsSuccessful() {
			rules = append(rules, v1alpha1.FailedRule{Name: r.Name, Type: r.RuleType.String(), Error: r.GetErrorString()})
		}
	}
	return rules
}

//ErrorRules returns error msgs from all rule
func (pi *PolicyInfo) ErrorRules() string {
	errorMsgs := []string{}
	for _, r := range pi.Rules {
		if !r.IsSuccessful() {
			errorMsgs = append(errorMsgs, r.ToString())
		}
	}
	return strings.Join(errorMsgs, ";")
}

type RuleType int

const (
	Mutation RuleType = iota
	Validation
	Generation
)

func (ri RuleType) String() string {
	return [...]string{
		"Mutation",
		"Validation",
		"Generation",
	}[ri]
}

//RuleInfo defines rule struct
type RuleInfo struct {
	Name     string
	Msgs     []string
	Changes  string // this will store the mutation patch being applied by the rule
	RuleType RuleType
	success  bool
}

//ToString reule information
//TODO: check if this is needed
func (ri *RuleInfo) ToString() string {
	str := "rulename: " + ri.Name
	msgs := strings.Join(ri.Msgs, ";")
	return strings.Join([]string{str, msgs}, ";")
}

//GetErrorString returns the error message for a rule
func (ri *RuleInfo) GetErrorString() string {
	return strings.Join(ri.Msgs, ";")
}

//NewRuleInfo creates a new RuleInfo
func NewRuleInfo(ruleName string, ruleType RuleType) *RuleInfo {
	return &RuleInfo{
		Name:     ruleName,
		Msgs:     []string{},
		RuleType: ruleType,
		success:  true, // fail to be set explicity
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
	if rules == nil {
		return
	}
	if !RulesSuccesfuly(rules) {
		pi.success = false
	}

	pi.Rules = append(pi.Rules, rules...)
}

//GetRuleNames gets the name of successful rules
func (pi *PolicyInfo) GetRuleNames(onSuccess bool) string {
	var ruleNames []string
	for _, rule := range pi.Rules {
		if onSuccess {
			if rule.IsSuccessful() {
				ruleNames = append(ruleNames, rule.Name)
			}
		} else {
			if !rule.IsSuccessful() {
				ruleNames = append(ruleNames, rule.Name)
			}
		}
	}

	return strings.Join(ruleNames, ",")
}

//ContainsRuleType checks if a policy info contains a rule type
func (pi *PolicyInfo) ContainsRuleType(ruleType RuleType) bool {
	for _, r := range pi.Rules {
		if r.RuleType == ruleType {
			return true
		}
	}
	return false
}
