package violation

// Mode to identify the CRUD event when the violation was identified
type Mode string

const (
	// Create resource
	Create Mode = "create"
	// Update resource
	Update Mode = "update"
	// Delete resource
	Delete Mode = "delete"
)

// ResourceMode to identify the source of violatino check
type ResourceMode string

const (
	// Resource type is kubernetes resource
	Resource ResourceMode = "resource"
	// Policy type is policy custom resource
	Policy ResourceMode = "policy"
)

type Target int

const (
	ResourceTarget Target = 1
	PolicyTarget   Target = 2
)

// Source for the events recorder
const violationEventSource = "policy-controller"

// Name for the workqueue to store the events
const workqueueViolationName = "Policy-Violations"

// Event Reason
const violationEventResrouce = "Violation"

type EventInfo struct {
	Resource       string
	Kind           string
	Reason         string
	Source         string
	ResourceTarget Target
}

// Info  input details
type Info struct {
	Kind     string
	Resource string
	Policy   string
	RuleName string
	Reason   string
}
