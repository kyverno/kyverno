package response

import (
	"fmt"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//EngineResponse engine response to the action
type EngineResponse struct {
	// Resource patched with the engine action changes
	PatchedResource unstructured.Unstructured

	// Original policy
	Policy *kyverno.ClusterPolicy

	// Policy Response
	PolicyResponse PolicyResponse
}

//PolicyResponse policy application response
type PolicyResponse struct {
	// policy details
	Policy PolicySpec `json:"policy"`
	// resource details
	Resource ResourceSpec `json:"resource"`
	// policy statistics
	PolicyStats `json:",inline"`
	// rule response
	Rules []RuleResponse `json:"rules"`
	// ValidationFailureAction: audit (default) or enforce
	ValidationFailureAction string

	ValidationFailureActionOverrides []ValidationFailureActionOverride
}

//PolicySpec policy
type PolicySpec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

//ResourceSpec resource action applied on
type ResourceSpec struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`

	// UID is not used to build the unique identifier
	// optional
	UID string `json:"uid"`
}

//GetKey returns the key
func (rs ResourceSpec) GetKey() string {
	return rs.Kind + "/" + rs.Namespace + "/" + rs.Name
}

//PolicyStats stores statistics for the single policy application
type PolicyStats struct {

	// time required to process the policy rules on a resource
	ProcessingTime time.Duration `json:"processingTime"`

	// Count of rules that were applied successfully
	RulesAppliedCount int `json:"rulesAppliedCount"`

	// Count of rules that with execution errors
	RulesErrorCount int `json:"rulesErrorCount"`

	// Timestamp of the instant the Policy was triggered
	PolicyExecutionTimestamp int64 `json:"policyExecutionTimestamp"`
}

//RuleResponse details for each rule application
type RuleResponse struct {

	// rule name specified in policy
	Name string `json:"name"`

	// rule type (Mutation,Generation,Validation) for Kyverno Policy
	Type string `json:"type"`

	// message response from the rule application
	Message string `json:"message"`

	// JSON patches, for mutation rules
	Patches [][]byte `json:"patches,omitempty"`

	// rule status
	Status RuleStatus `json:"status"`

	// statistics
	RuleStats `json:",inline"`
}

//ToString ...
func (rr RuleResponse) ToString() string {
	return fmt.Sprintf("rule %s (%s): %v", rr.Name, rr.Type, rr.Message)
}

//RuleStats stores the statistics for the single rule application
type RuleStats struct {
	// time required to apply the rule on the resource
	ProcessingTime time.Duration `json:"processingTime"`
	// Timestamp of the instant the rule got triggered
	RuleExecutionTimestamp int64 `json:"ruleExecutionTimestamp"`
}

//IsSuccessful checks if any rule has failed or not
func (er EngineResponse) IsSuccessful() bool {
	for _, r := range er.PolicyResponse.Rules {
		if r.Status == RuleStatusFail || r.Status == RuleStatusError {
			return false
		}
	}

	return true
}

//IsFailed checks if any rule has succeeded or not
func (er EngineResponse) IsFailed() bool {
	for _, r := range er.PolicyResponse.Rules {
		if r.Status == RuleStatusFail {
			return true
		}
	}

	return false
}

//GetPatches returns all the patches joined
func (er EngineResponse) GetPatches() [][]byte {
	var patches [][]byte
	for _, r := range er.PolicyResponse.Rules {
		if r.Patches != nil {
			patches = append(patches, r.Patches...)
		}
	}

	return patches
}

//GetFailedRules returns failed rules
func (er EngineResponse) GetFailedRules() []string {
	return er.getRules(RuleStatusFail)
}

//GetSuccessRules returns success rules
func (er EngineResponse) GetSuccessRules() []string {
	return er.getRules(RuleStatusPass)
}

// GetResourceSpec returns resourceSpec of er
func (er EngineResponse) GetResourceSpec() ResourceSpec {
	return ResourceSpec{
		Kind:       er.PatchedResource.GetKind(),
		APIVersion: er.PatchedResource.GetAPIVersion(),
		Namespace:  er.PatchedResource.GetNamespace(),
		Name:       er.PatchedResource.GetName(),
		UID:        string(er.PatchedResource.GetUID()),
	}
}

func (er EngineResponse) getRules(status RuleStatus) []string {
	var rules []string
	for _, r := range er.PolicyResponse.Rules {
		if r.Status == status {
			rules = append(rules, r.Name)
		}
	}

	return rules
}

type ValidationFailureActionOverride struct {
	Action     string   `json:"action"`
	Namespaces []string `json:"namespaces"`
}
