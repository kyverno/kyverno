package metrics

type PolicyValidationMode string

const (
	Enforce PolicyValidationMode = "enforce"
	Audit   PolicyValidationMode = "audit"
)

type PolicyType string

const (
	Cluster    PolicyType = "cluster"
	Namespaced PolicyType = "namespaced"
)

type PolicyBackgroundMode string

const (
	BackgroundTrue  PolicyBackgroundMode = "true"
	BackgroundFalse PolicyBackgroundMode = "false"
)

type RuleType string

const (
	Validate      RuleType = "validate"
	Mutate        RuleType = "mutate"
	Generate      RuleType = "generate"
	EmptyRuleType RuleType = "-"
)

type RuleResult string

const (
	Pass  RuleResult = "pass"
	Fail  RuleResult = "fail"
	Warn  RuleResult = "warn"
	Error RuleResult = "error"
	Skip  RuleResult = "skip"
)

type RuleExecutionCause string

const (
	AdmissionRequest RuleExecutionCause = "admission_request"
	BackgroundScan   RuleExecutionCause = "background_scan"
)

type ResourceRequestOperation string

const (
	ResourceCreated   ResourceRequestOperation = "create"
	ResourceUpdated   ResourceRequestOperation = "update"
	ResourceDeleted   ResourceRequestOperation = "delete"
	ResourceConnected ResourceRequestOperation = "connect"
)
