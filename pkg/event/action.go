package event

// Action types of Event Actions
type Action string

const (
	ResourceBlocked   Action = "Resource Blocked"
	ResourcePassed    Action = "Resource Passed"
	ResourceGenerated Action = "Resource Generated"
	ResourceMutated   Action = "Resource Mutated"
	ResourceCleanedUp Action = "Resource Cleaned Up"
	None              Action = "None"
)
