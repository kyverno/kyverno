package engine

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//EngineResponse engine response to the action
type EngineResponse struct {
	// Resource patched with the engine action changes
	PatchedResource unstructured.Unstructured
	// Policy Response
	PolicyResponse PolicyResponse
}

//PolicyResponse policy application response
type PolicyResponse struct {
	// policy name
	Policy string `json:"policy"`
	// resource details
	Resource ResourceSpec `json:"resource"`
	// policy statistics
	PolicyStats `json:",inline"`
	// rule response
	Rules []RuleResponse `json:"rules"`
	// ValidationFailureAction: audit(default if not set),enforce
	ValidationFailureAction string
}

//ResourceSpec resource action applied on
type ResourceSpec struct {
	//TODO: support ApiVersion
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

//GetKey returns the key
func (rs ResourceSpec) GetKey() string {
	return rs.Kind + "/" + rs.Namespace + "/" + rs.Name
}

//PolicyStats stores statistics for the single policy application
type PolicyStats struct {
	// time required to process the policy rules on a resource
	ProcessingTime time.Duration `json:"processingTime"`
	// Count of rules that were applied succesfully
	RulesAppliedCount int `json:"rulesAppliedCount"`
}

//RuleResponse details for each rule applicatino
type RuleResponse struct {
	// rule name specified in policy
	Name string `json:"name"`
	// rule type (Mutation,Generation,Validation) for Kyverno Policy
	Type string `json:"type"`
	// message response from the rule application
	Message string `json:"message"`
	// JSON patches, for mutation rules
	Patches [][]byte `json:"patches,omitempty"`
	// success/fail
	Success bool `json:"success"`
	// statistics
	RuleStats `json:",inline"`
}

//ToString ...
func (rr RuleResponse) ToString() string {
	return fmt.Sprintf("rule %s (%s): %v", rr.Name, rr.Type, rr.Message)
}

//RuleStats stores the statisctis for the single rule application
type RuleStats struct {
	// time required to appliy the rule on the resource
	ProcessingTime time.Duration `json:"processingTime"`
}

//IsSuccesful checks if any rule has failed or not
func (er EngineResponse) IsSuccesful() bool {
	for _, r := range er.PolicyResponse.Rules {
		if !r.Success {
			return false
		}
	}
	return true
}

//GetPatches returns all the patches joined
func (er EngineResponse) GetPatches() [][]byte {
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
func (er EngineResponse) GetFailedRules() []string {
	return er.getRules(false)
}

//GetSuccessRules returns success rules
func (er EngineResponse) GetSuccessRules() []string {
	return er.getRules(true)
}

func (er EngineResponse) getRules(success bool) []string {
	var rules []string
	for _, r := range er.PolicyResponse.Rules {
		if r.Success == success {
			rules = append(rules, r.Name)
		}
	}
	return rules
}
