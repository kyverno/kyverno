package autogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyRewritesExpressions(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		config string
		want   string
	}{
		{
			name:   "deployments spec",
			expr:   "object.spec",
			config: "deployments",
			want:   "object.spec.template.spec",
		},
		{
			name:   "deployments oldObject spec",
			expr:   "oldObject.spec",
			config: "deployments",
			want:   "oldObject.spec.template.spec",
		},
		{
			name:   "cronjobs spec",
			expr:   "object.spec",
			config: "cronjobs",
			want:   "object.spec.jobTemplate.spec.template.spec",
		},
		{
			name:   "cronjobs oldObject spec",
			expr:   "oldObject.spec",
			config: "cronjobs",
			want:   "oldObject.spec.jobTemplate.spec.template.spec",
		},
		{
			name:   "deployments metadata is rewritten",
			expr:   "object.metadata",
			config: "deployments",
			want:   "object.spec.template.metadata",
		},
		{
			name:   "deployments metadata.labels is rewritten",
			expr:   "object.metadata.labels['app']",
			config: "deployments",
			want:   "object.spec.template.metadata.labels['app']",
		},
		{
			name:   "deployments object metadata.namespace is preserved",
			expr:   "object.metadata.namespace",
			config: "deployments",
			want:   "object.metadata.namespace",
		},
		{
			name:   "deployments oldObject metadata.namespace is preserved",
			expr:   "oldObject.metadata.namespace",
			config: "deployments",
			want:   "oldObject.metadata.namespace",
		},
		{
			name:   "cronjobs object metadata.namespace is preserved",
			expr:   "object.metadata.namespace",
			config: "cronjobs",
			want:   "object.metadata.namespace",
		},
		{
			name:   "cronjobs oldObject metadata.namespace is preserved",
			expr:   "oldObject.metadata.namespace",
			config: "cronjobs",
			want:   "oldObject.metadata.namespace",
		},
		{
			name:   "namespace membership expression is preserved (deployments)",
			expr:   "!(object.metadata.namespace in ['opencost', 'kube-system'])",
			config: "deployments",
			want:   "!(object.metadata.namespace in ['opencost', 'kube-system'])",
		},
		{
			name:   "namespace membership expression is preserved (cronjobs)",
			expr:   "!(object.metadata.namespace in ['opencost', 'kube-system'])",
			config: "cronjobs",
			want:   "!(object.metadata.namespace in ['opencost', 'kube-system'])",
		},
		{
			name:   "namespace preserved while sibling metadata fields are rewritten",
			expr:   "object.metadata.namespace == 'foo' && object.metadata.labels['team'] == 'platform'",
			config: "deployments",
			want:   "object.metadata.namespace == 'foo' && object.spec.template.metadata.labels['team'] == 'platform'",
		},
		{
			name:   "only the namespace segment is protected, not longer identifiers",
			expr:   "object.metadata.namespaceFoo",
			config: "deployments",
			want:   "object.spec.template.metadata.namespaceFoo",
		},
		{
			name:   "user content containing protected sentinel-like text is not corrupted",
			expr:   "object.metadata.labels['__KYVERNO_PROTECTED_OBJECT_METADATA_NAMESPACE__'] == 'x'",
			config: "deployments",
			want:   "object.spec.template.metadata.labels['__KYVERNO_PROTECTED_OBJECT_METADATA_NAMESPACE__'] == 'x'",
		},
		{
			name:   "cronjobs containers expression",
			expr:   "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
			config: "cronjobs",
			want:   "object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
		},
		{
			name:   "deployments containers expression",
			expr:   "object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
			config: "deployments",
			want:   "object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply([]byte(tt.expr), ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
			assert.Equal(t, []byte(tt.want), got)
		})
	}
}

func TestApplyCELRewritesExpressions(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		config string
		want   string
	}{
		{
			name:   "deployments object metadata.namespace is preserved (bracket syntax)",
			expr:   "object.metadata['namespace'] == 'foo'",
			config: "deployments",
			want:   "object.metadata['namespace'] == 'foo'",
		},
		{
			name:   "deployments object metadata.namespace is preserved (double-quote bracket syntax)",
			expr:   "object.metadata[\"namespace\"] == 'foo'",
			config: "deployments",
			want:   "object.metadata[\"namespace\"] == 'foo'",
		},
		{
			name:   "string literals are not rewritten",
			expr:   "object.metadata.name == 'object.spec'",
			config: "deployments",
			want:   "object.spec.template.metadata.name == 'object.spec'",
		},
		{
			name:   "triple quoted string literals are not rewritten",
			expr:   "object.metadata.name == '''object.spec'''",
			config: "deployments",
			want:   "object.spec.template.metadata.name == '''object.spec'''",
		},
		{
			name:   "triple double quoted string literals are not rewritten",
			expr:   "object.metadata.name == \"\"\"object.spec\"\"\"",
			config: "deployments",
			want:   "object.spec.template.metadata.name == \"\"\"object.spec\"\"\"",
		},
		{
			name:   "raw single quoted string literals are not rewritten",
			expr:   "object.metadata.name == r'object.spec'",
			config: "deployments",
			want:   "object.spec.template.metadata.name == r'object.spec'",
		},
		{
			name:   "backtick string literals are not rewritten",
			expr:   "object.metadata.name == `object.spec`",
			config: "deployments",
			want:   "object.spec.template.metadata.name == `object.spec`",
		},
		{
			name:   "escaped string literals are handled correctly",
			expr:   "object.metadata.name == 'object\\'spec' && object.spec.replicas > 0",
			config: "deployments",
			want:   "object.spec.template.metadata.name == 'object\\'spec' && object.spec.template.spec.replicas > 0",
		},
		{
			name:   "prefix matches are skipped (e.g. xobject.spec)",
			expr:   "xobject.spec == true",
			config: "deployments",
			want:   "xobject.spec == true",
		},
		{
			name:   "field selects are skipped (e.g. a.object.spec)",
			expr:   "a.object.spec == true",
			config: "deployments",
			want:   "a.object.spec == true",
		},
		{
			name:   "suffix matches are skipped (e.g. object.specification)",
			expr:   "object.specification == true",
			config: "deployments",
			want:   "object.specification == true",
		},
		{
			name:   "request.object is rewritten",
			expr:   "request.object.spec.replicas > 0",
			config: "deployments",
			want:   "request.object.spec.template.spec.replicas > 0",
		},
		{
			name:   "request.oldObject is rewritten",
			expr:   "request.oldObject.spec.replicas > 0",
			config: "deployments",
			want:   "request.oldObject.spec.template.spec.replicas > 0",
		},
		{
			name:   "myrequest.object is skipped",
			expr:   "myrequest.object.spec.replicas > 0",
			config: "deployments",
			want:   "myrequest.object.spec.replicas > 0",
		},
		{
			name:   "equivalent selector forms (single quote bracket) are rewritten",
			expr:   "object['spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "equivalent selector forms (double quote bracket) are rewritten",
			expr:   "object[\"spec\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "JSON-escaped namespace paths are preserved",
			expr:   "object.metadata[\\\"namespace\\\"] == 'foo'",
			config: "deployments",
			want:   "object.metadata[\\\"namespace\\\"] == 'foo'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyCEL([]byte(tt.expr), ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
			assert.Equal(t, []byte(tt.want), got)
		})
	}
}
