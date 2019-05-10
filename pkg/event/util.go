package event

const eventSource = "policy-controller"

const eventWorkQueueName = "policy-controller-events"

const eventWorkerThreadCount = 1

type eventInfo struct {
	Kind     string
	Resource string
	Reason   string
	Message  string
}

//MsgKey is an identified to determine the preset message formats
type MsgKey int

const (
	FResourcePolcy MsgKey = iota
	FProcessRule
	SPolicyApply
	SRuleApply
	FPolicyApplyBlockCreate
	FPolicyApplyBlockUpdate
	FPolicyApplyBlockUpdateRule
)
