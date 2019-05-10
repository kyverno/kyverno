package violation

import policytype "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"

// Source for the events recorder
const violationEventSource = "policy-controller"

// Name for the workqueue to store the events
const workqueueViolationName = "Policy-Violations"

// Event Reason
const violationEventResrouce = "Violation"

type ViolationInfo struct {
	Policy string
	policytype.Violation
}
