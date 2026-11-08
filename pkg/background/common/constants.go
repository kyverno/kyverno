package common

const (
	GeneratePolicyLabel          = "generate.kyverno.io/policy-name"
	GeneratePolicyNamespaceLabel = "generate.kyverno.io/policy-namespace"
	GenerateRuleLabel            = "generate.kyverno.io/rule-name"
	GenerateTriggerNameLabel     = "generate.kyverno.io/trigger-name"
	GenerateTriggerUIDLabel      = "generate.kyverno.io/trigger-uid"
	GenerateTriggerNSLabel       = "generate.kyverno.io/trigger-namespace"
	GenerateTriggerKindLabel     = "generate.kyverno.io/trigger-kind"
	GenerateTriggerVersionLabel  = "generate.kyverno.io/trigger-version"
	GenerateTriggerGroupLabel    = "generate.kyverno.io/trigger-group"
	GenerateSourceNameLabel      = "generate.kyverno.io/source-name"
	GenerateSourceUIDLabel       = "generate.kyverno.io/source-uid"
	GenerateSourceNSLabel        = "generate.kyverno.io/source-namespace"
	GenerateSourceKindLabel      = "generate.kyverno.io/source-kind"
	GenerateSourceVersionLabel   = "generate.kyverno.io/source-version"
	GenerateSourceGroupLabel     = "generate.kyverno.io/source-group"
	GenerateTypeCloneSourceLabel = "generate.kyverno.io/clone-source"
)

// CEL generators store scope explicitly so the same bare policy name can be
// used by a cluster-scoped and a namespaced policy without ownership collision.
// These labels are distinct from the legacy generate policy-namespace label.
const (
	GeneratePolicyScopeLabel      = "generate.kyverno.io/policy-scope"
	GeneratePolicyScopeCluster    = "cluster"
	GeneratePolicyScopeNamespaced = "namespaced"
)

// MatchesPolicyTriggerNamespace checks that a namespaced policy owns the trigger.
// In addition to namespaced resources, policy controllers allow a policy to
// select its own cluster-scoped Namespace (metadata.namespace is empty).
func MatchesPolicyTriggerNamespace(labels map[string]string, policyNamespace string) bool {
	if policyNamespace == "" {
		return false
	}
	if labels[GenerateTriggerNSLabel] == policyNamespace {
		return true
	}
	return labels[GenerateTriggerNSLabel] == "" &&
		labels[GenerateTriggerGroupLabel] == "" &&
		labels[GenerateTriggerKindLabel] == "Namespace" &&
		labels[GenerateTriggerNameLabel] == policyNamespace
}
