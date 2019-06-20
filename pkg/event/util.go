package event

const (
	eventSource            = "policy-controller"
	eventWorkQueueName     = "policy-controller-events"
	eventWorkerThreadCount = 1
	policyKind             = "Policy"
)

//Info defines the event details
type Info struct {
	// Kind is the kind of the resource
	Kind string
	// Resource is namespace/name,
	// namespace is optional if the resource is non-namepsaced
	Resource string
	Reason   string
	Message  string
}

//MsgKey is an identified to determine the preset message formats
type MsgKey int

//Message id for pre-defined messages
const (
	FProcessPolicy MsgKey = iota
	FProcessRule
	SPolicyApply
	SRuleApply
	FPolicyApplyBlockCreate
	FPolicyApplyBlockUpdate
	FPolicyApplyBlockUpdateRule
)
