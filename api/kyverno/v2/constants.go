package v2

const (
	// URMutatePolicyLabel adds the policy name to URs for mutate policies
	URMutatePolicyLabel            = "mutate.updaterequest.kyverno.io/policy-name"
	URMutateTriggerNameLabel       = "mutate.updaterequest.kyverno.io/trigger-name"
	URMutateTriggerNSLabel         = "mutate.updaterequest.kyverno.io/trigger-namespace"
	URMutateTriggerKindLabel       = "mutate.updaterequest.kyverno.io/trigger-kind"
	URMutateTriggerAPIVersionLabel = "mutate.updaterequest.kyverno.io/trigger-apiversion"

	// URGeneratePolicyLabel adds the policy name to URs for generate policies
	URGeneratePolicyLabel          = "generate.kyverno.io/policy-name"
	URGenerateRetryCountAnnotation = "generate.kyverno.io/retry-count"
)
