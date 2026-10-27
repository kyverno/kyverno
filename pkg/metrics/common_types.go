package metrics

type PolicyBackgroundMode string

const (
	BackgroundTrue  PolicyBackgroundMode = "true"
	BackgroundFalse PolicyBackgroundMode = "false"
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
	DynamicClient      ClientType = "dynamic"
	KubeClient         ClientType = "kubeclient"
	KyvernoClient      ClientType = "kyverno"
	MetadataClient     ClientType = "metadata"
	ApiServerClient    ClientType = "apiserver"
	PolicyReportClient ClientType = "policyreport"
)
