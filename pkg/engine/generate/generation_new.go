package generate

// func filterRules(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo request.RequestInfo) *response.EngineResponse {
// 	resp := &response.EngineResponse{}

// 	for _, rule := range policy.Spec.Rules {
// 		if !rule.HasGenerate() {
// 			continue
// 		}
// 		if !rbac.MatchAdmissionInfo(rule, admissionInfo) {
// 			continue
// 		}
// 		if
// 	}

// 	return resp
// }
