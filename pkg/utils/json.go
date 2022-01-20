package utils

import (
	"encoding/json"
	"reflect"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

// JoinPatches joins array of serialized JSON patches to the single JSONPatch array
func JoinPatches(patches [][]byte) []byte {
	var result []byte
	if len(patches) == 0 {
		return result
	}

	result = append(result, []byte("[\n")...)
	for index, patch := range patches {
		result = append(result, patch...)
		if index != len(patches)-1 {
			result = append(result, []byte(",\n")...)
		}
	}
	result = append(result, []byte("\n]")...)
	return result
}

// MarshalPolicy accurately marshals a policy to JSON,
// normal marshal would cause empty sub structs in
// policy to be non nil.
// TODO This needs to be removed. A simpler way to encode and decode Policy is needed.
func MarshalPolicy(policy v1.ClusterPolicy) []byte {
	var rules []interface{}
	rulesRaw, _ := json.Marshal(policy.Spec.Rules)
	_ = json.Unmarshal(rulesRaw, &rules)
	for i, r := range rules {
		rule, _ := r.(map[string]interface{})

		if reflect.DeepEqual(policy.Spec.Rules[i].Mutation, v1.Mutation{}) {
			delete(rule, "mutate")
		}
		if reflect.DeepEqual(policy.Spec.Rules[i].Validation, v1.Validation{}) {
			delete(rule, "validate")
		}
		if reflect.DeepEqual(policy.Spec.Rules[i].Generation, v1.Generation{}) {
			delete(rule, "generate")
		}

		rules[i] = rule
	}

	var policyRepresentation = make(map[string]interface{})
	policyRaw, _ := json.Marshal(policy)
	_ = json.Unmarshal(policyRaw, &policyRepresentation)

	specRepresentation, _ := policyRepresentation["spec"].(map[string]interface{})

	specRepresentation["rules"] = rules

	policyRepresentation["spec"] = specRepresentation

	policyRaw, _ = json.Marshal(policyRepresentation)

	return policyRaw
}
