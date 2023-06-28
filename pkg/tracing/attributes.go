package tracing

import (
	"go.opentelemetry.io/otel/attribute"
)

const (
	limit = 256
	// engine attributes
	PolicyGroupKey     = attribute.Key("kyverno.policy.group")
	PolicyVersionKey   = attribute.Key("kyverno.policy.version")
	PolicyKindKey      = attribute.Key("kyverno.policy.kind")
	PolicyNameKey      = attribute.Key("kyverno.policy.name")
	PolicyNamespaceKey = attribute.Key("kyverno.policy.namespace")
	RuleNameKey        = attribute.Key("kyverno.rule.name")
	// admission resource attributes
	// ResourceNameKey       = attribute.Key("admission.resource.name")
	// ResourceNamespaceKey  = attribute.Key("admission.resource.namespace")
	// ResourceGroupKey      = attribute.Key("admission.resource.group")
	// ResourceVersionKey    = attribute.Key("admission.resource.version")
	// ResourceKindKey       = attribute.Key("admission.resource.kind")
	// ResourceUidKey        = attribute.Key("admission.resource.uid")
	// admission request attributes
	RequestNameKey                    = attribute.Key("admission.request.name")
	RequestNamespaceKey               = attribute.Key("admission.request.namespace")
	RequestUidKey                     = attribute.Key("admission.request.uid")
	RequestOperationKey               = attribute.Key("admission.request.operation")
	RequestDryRunKey                  = attribute.Key("admission.request.dryrun")
	RequestKindGroupKey               = attribute.Key("admission.request.kind.group")
	RequestKindVersionKey             = attribute.Key("admission.request.kind.version")
	RequestKindKindKey                = attribute.Key("admission.request.kind.kind")
	RequestSubResourceKey             = attribute.Key("admission.request.subresource")
	RequestRequestKindGroupKey        = attribute.Key("admission.request.requestkind.group")
	RequestRequestKindVersionKey      = attribute.Key("admission.request.requestkind.version")
	RequestRequestKindKindKey         = attribute.Key("admission.request.requestkind.kind")
	RequestRequestSubResourceKey      = attribute.Key("admission.request.requestsubresource")
	RequestResourceGroupKey           = attribute.Key("admission.request.resource.group")
	RequestResourceVersionKey         = attribute.Key("admission.request.resource.version")
	RequestResourceResourceKey        = attribute.Key("admission.request.resource.resource")
	RequestRequestResourceGroupKey    = attribute.Key("admission.request.requestresource.group")
	RequestRequestResourceVersionKey  = attribute.Key("admission.request.requestresource.version")
	RequestRequestResourceResourceKey = attribute.Key("admission.request.requestresource.resource")
	RequestUserNameKey                = attribute.Key("admission.request.user.name")
	RequestUserUidKey                 = attribute.Key("admission.request.user.uid")
	RequestUserGroupsKey              = attribute.Key("admission.request.user.groups")
	RequestRolesKey                   = attribute.Key("admission.request.roles")
	RequestClusterRolesKey            = attribute.Key("admission.request.clusterroles")
	RequestGroupKey                   = attribute.Key("admission.request.group")
	RequestVersionKey                 = attribute.Key("admission.request.version")
	RequestKindKey                    = attribute.Key("admission.request.kind")
	RequestFilteredKey                = attribute.Key("admission.request.filtered")
	// admission response attributes
	ResponseUidKey           = attribute.Key("admission.response.uid")
	ResponseAllowedKey       = attribute.Key("admission.response.allowed")
	ResponseWarningsKey      = attribute.Key("admission.response.warnings")
	ResponseResultStatusKey  = attribute.Key("admission.response.result.status")
	ResponseResultMessageKey = attribute.Key("admission.response.result.message")
	ResponseResultReasonKey  = attribute.Key("admission.response.result.reason")
	ResponseResultCodeKey    = attribute.Key("admission.response.result.code")
	ResponsePatchTypeKey     = attribute.Key("admission.response.patchtype")
	// kube client attributes
	KubeClientGroupKey     = attribute.Key("kube.client.group")
	KubeClientKindKey      = attribute.Key("kube.client.kind")
	KubeClientOperationKey = attribute.Key("kube.client.operation")
	KubeClientNamespaceKey = attribute.Key("kube.client.namespace")
)

// StringValue truncates the input value if its size is above the limit.
// Some backends impose a limit on the size of a tag value.
func StringValue(value string) string {
	if len(value) > limit {
		value = value[:limit-3]
		value += "..."
	}
	return value
}
