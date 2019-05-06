package violation

// Source for the events recorder
const violationEventSource = "policy-controller"

// Name for the workqueue to store the events
const workqueueViolationName = "Policy-Violations"

// Event Reason
const violationEventResrouce = "Violation"

// Info  input details
type Info struct {
	Kind     string
	Resource string
	Policy   string
	RuleName string
	Reason   string
}
