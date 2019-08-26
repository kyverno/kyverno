package engine

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//EngineResponseNew engine response to the action
type EngineResponseNew struct {
	// Resource patched with the engine action changes
	PatchedResource unstructured.Unstructured
	// Policy Response
	PolicyResponse PolicyResponse
}

//PolicyResponse policy application response
type PolicyResponse struct {
	// policy name
	Policy string
	// resource details
	Resource ResourceSpec
	// policy statistics
	PolicyStats
	// rule response
	Rules []RuleResponse
	// ValidationFailureAction: audit,enforce(default)
	ValidationFailureAction string
}

//ResourceSpec resource action applied on
type ResourceSpec struct {
	//TODO: support ApiVersion
	Kind       string
	APIVersion string
	Namespace  string
	Name       string
}

//PolicyStats stores statistics for the single policy application
type PolicyStats struct {
	// time required to process the policy rules on a resource
	ProcessingTime time.Duration
	// Count of rules that were applied succesfully
	RulesAppliedCount int
}

//RuleResponse details for each rule applicatino
type RuleResponse struct {
	// rule name specified in policy
	Name string
	// rule type (Mutation,Generation,Validation) for Kyverno Policy
	Type string
	// message response from the rule application
	Message string
	// JSON patches, for mutation rules
	Patches [][]byte
	// success/fail
	Success bool
	// statistics
	RuleStats
}

//ToString ...
func (rr RuleResponse) ToString() string {
	return fmt.Sprintf("rule %s (%s): %v", rr.Name, rr.Type, rr.Message)
}

//RuleStats stores the statisctis for the single rule application
type RuleStats struct {
	// time required to appliy the rule on the resource
	ProcessingTime time.Duration
}

//IsSuccesful checks if any rule has failed or not
func (er EngineResponseNew) IsSuccesful() bool {
	for _, r := range er.PolicyResponse.Rules {
		if !r.Success {
			return false
		}
	}
	return true
}

//GetPatches returns all the patches joined
func (er EngineResponseNew) GetPatches() [][]byte {
	var patches [][]byte
	for _, r := range er.PolicyResponse.Rules {
		if r.Patches != nil {
			patches = append(patches, r.Patches...)
		}
	}
	// join patches
	return patches
}

//GetFailedRules returns failed rules
func (er EngineResponseNew) GetFailedRules() []string {
	return er.getRules(false)
}

//GetSuccessRules returns success rules
func (er EngineResponseNew) GetSuccessRules() []string {
	return er.getRules(true)
}

func (er EngineResponseNew) getRules(success bool) []string {
	var rules []string
	for _, r := range er.PolicyResponse.Rules {
		if r.Success == success {
			rules = append(rules, r.Name)
		}
	}
	return rules
}
