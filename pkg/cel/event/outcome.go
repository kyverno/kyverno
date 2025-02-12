package event

// EventOutcome represents the outcome of policy evaluation.
type EventOutcome string

const (
	OutcomeViolate EventOutcome = "Violate"
	OutcomeError   EventOutcome = "Error"
	OutcomeSkip    EventOutcome = "Skip"
	OutcomePass    EventOutcome = "Pass"
)
