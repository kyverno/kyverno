package event

const eventSource = "policy-controller"

const eventWorkQueueName = "policy-controller-events"

const eventWorkerThreadCount = 1

//Info defines the event details
type Info struct {
	Kind     string
	Resource string
	Reason   string
	Message  string
}

//MsgKey is an identified to determine the preset message formats
type MsgKey int

//Message id for pre-defined messages
const (
	FResourcePolcy MsgKey = iota
	FProcessRule
	SPolicyApply
	SRulesApply
	FPolicyApplyBlockCreate
	FPolicyApplyBlockUpdate
	FPolicyApplyBlockUpdateRule
)
