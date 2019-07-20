package violation

import policytype "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"

// Source for the events recorder
const violationEventSource = "policy-controller"

// Name for the workqueue to store the events
const workqueueViolationName = "Policy-Violations"

// Event Reason
const violationEventResrouce = "Violation"

//Info describes the policyviolation details
type Info struct {
	Policy string
	policytype.Violation
}

func (i Info) getKey() string {
	return i.Kind + "/" + i.Namespace + "/" + i.Name
}

//BuildKey returns the key format
func BuildKey(rKind, rNs, rName string) string {
	return rKind + "/" + rNs + "/" + rName
}
