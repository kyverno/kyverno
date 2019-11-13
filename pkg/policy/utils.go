package policy

import kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

// reEvaulatePolicy checks if the policy needs to be re-evaulated
// during re-evaulation we remove all the old policy violations and re-create new ones
// - Rule count changes
// - Rule resource description changes
// - Rule operation changes
// - Rule name changed
func reEvaulatePolicy(curP, oldP *kyverno.ClusterPolicy) bool {
	// count of rules changed
	if len(curP.Spec.Rules) != len(curP.Spec.Rules) {

	}
	return true
}
