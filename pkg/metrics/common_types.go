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
	ImageVerify   RuleType = "imageVerify"
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

type ClientQueryOperation string

const (
	ClientCreate           ClientQueryOperation = "create"
	ClientGet              ClientQueryOperation = "get"
	ClientList             ClientQueryOperation = "list"
	ClientUpdate           ClientQueryOperation = "update"
	ClientUpdateStatus     ClientQueryOperation = "update_status"
	ClientDelete           ClientQueryOperation = "delete"
	ClientDeleteCollection ClientQueryOperation = "delete_collection"
	ClientWatch            ClientQueryOperation = "watch"
	ClientPatch            ClientQueryOperation = "patch"
)

type ClientType string

const (
	KubeDynamicClient  ClientType = "dynamic"
	KubeClient         ClientType = "kubeclient"
	KyvernoClient      ClientType = "kyverno"
	PolicyReportClient ClientType = "policyreport"
)
