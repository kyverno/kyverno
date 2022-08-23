package v1beta1

const (
	// URMutatePolicyLabel adds the policy name to URs for mutate policies
	URMutatePolicyLabel            = "mutate.updaterequest.kyverno.io/policy-name"
	URMutateTriggerNameLabel       = "mutate.updaterequest.kyverno.io/trigger-name"
	URMutateTriggerNSLabel         = "mutate.updaterequest.kyverno.io/trigger-namespace"
	URMutatetriggerKindLabel       = "mutate.updaterequest.kyverno.io/trigger-kind"
	URMutatetriggerAPIVersionLabel = "mutate.updaterequest.kyverno.io/trigger-apiversion"

	// URGeneratePolicyLabel adds the policy name to URs for generate policies
	URGeneratePolicyLabel          = "generate.kyverno.io/policy-name"
	URGenerateResourceNameLabel    = "generate.kyverno.io/resource-name"
	URGenerateResourceNSLabel      = "generate.kyverno.io/resource-namespace"
	URGenerateResourceKindLabel    = "generate.kyverno.io/resource-kind"
	URGenerateRetryCountAnnotation = "generate.kyverno.io/retry-count"
)
