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
			name:   "deployments bare metadata is preserved",
			expr:   "object.metadata",
			config: "deployments",
			want:   "object.metadata",
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
			name:   "metadata identifiers are not rewritten",
			expr:   "object.metadata.namespaceFoo",
			config: "deployments",
			want:   "object.metadata.namespaceFoo",
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
		{
			name:   "deployments object metadata.name is preserved",
			expr:   "object.metadata.name",
			config: "deployments",
			want:   "object.metadata.name",
		},
		{
			name:   "cronjobs object metadata.name is preserved",
			expr:   "object.metadata.name",
			config: "cronjobs",
			want:   "object.metadata.name",
		},
		{
			name:   "deployments object metadata.uid is preserved",
			expr:   "object.metadata.uid",
			config: "deployments",
			want:   "object.metadata.uid",
		},
		{
			name:   "deployments object metadata.generateName is preserved",
			expr:   "object.metadata.generateName",
			config: "deployments",
			want:   "object.metadata.generateName",
		},
		{
			name:   "deployments resource.Get preserves namespace and name",
			expr:   "resource.Get('v1', 'secrets', object.metadata.namespace, object.metadata.name)",
			config: "deployments",
			want:   "resource.Get('v1', 'secrets', object.metadata.namespace, object.metadata.name)",
		},
		{
			name:   "cronjobs resource.Get preserves namespace and name",
			expr:   "resource.Get('v1', 'secrets', object.metadata.namespace, object.metadata.name)",
			config: "cronjobs",
			want:   "resource.Get('v1', 'secrets', object.metadata.namespace, object.metadata.name)",
		},
		{
			name:   "deployments name preserved while labels are rewritten",
			expr:   "object.metadata.name == 'x' && object.metadata.labels['team'] == 'y'",
			config: "deployments",
			want:   "object.metadata.name == 'x' && object.spec.template.metadata.labels['team'] == 'y'",
		},
		{
			name:   "deployments bracket notation metadata name is preserved",
			expr:   `object.metadata["name"]`,
			config: "deployments",
			want:   `object.metadata["name"]`,
		},
		{
			name:   "deployments single-quote bracket notation metadata name is preserved",
			expr:   "object.metadata['name']",
			config: "deployments",
			want:   "object.metadata['name']",
		},
		{
			name:   "deployments bracket notation metadata labels is rewritten",
			expr:   `object.metadata["labels"]`,
			config: "deployments",
			want:   `object.spec.template.metadata["labels"]`,
		},
		{
			name:   "metadata name identifiers are not rewritten",
			expr:   "object.metadata.nameFoo",
			config: "deployments",
			want:   "object.metadata.nameFoo",
		},
		{
			name:   "statefulsets object metadata.name is preserved",
			expr:   "object.metadata.name",
			config: "statefulsets",
			want:   "object.metadata.name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply([]byte(tt.expr), ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
			assert.Equal(t, []byte(tt.want), got)
		})
	}
}

func TestApplyMetadataRewriteBoundariesAndIdempotence(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		config string
		want   string
	}{
		{
			name:   "old object annotations rewrite",
			expr:   `oldObject.metadata["annotations"]["example.com/key"]`,
			config: "cronjobs",
			want:   `oldObject.spec.jobTemplate.spec.template.metadata["annotations"]["example.com/key"]`,
		},
		{
			name:   "repeated label paths rewrite",
			expr:   "object.metadata.labels == oldObject.metadata.labels",
			config: "deployments",
			want:   "object.spec.template.metadata.labels == oldObject.spec.template.metadata.labels",
		},
		{
			name:   "optional labels selector rewrites",
			expr:   "object.metadata.?labels[?'app'].orValue('default')",
			config: "deployments",
			want:   "object.spec.template.metadata.?labels[?'app'].orValue('default')",
		},
		{
			name:   "mixed controller and template metadata",
			expr:   "object.metadata.name + object.metadata.labels.app",
			config: "deployments",
			want:   "object.metadata.name + object.spec.template.metadata.labels.app",
		},
		{
			name:   "future metadata field is preserved",
			expr:   "object.metadata.futureField",
			config: "deployments",
			want:   "object.metadata.futureField",
		},
		{
			name:   "labels identifier suffix is preserved",
			expr:   "object.metadata.labelsFoo",
			config: "deployments",
			want:   "object.metadata.labelsFoo",
		},
		{
			name:   "already rewritten paths remain unchanged",
			expr:   "object.spec.template.metadata.labels",
			config: "deployments",
			want:   "object.spec.template.metadata.labels",
		},
		{
			name:   "cel string literals retain existing byte replacement behavior",
			expr:   `"object.metadata.labels"`,
			config: "deployments",
			want:   `"object.spec.template.metadata.labels"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replacements := ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]
			once := Apply([]byte(tt.expr), replacements...)
			twice := Apply(once, replacements...)
			assert.Equal(t, []byte(tt.want), once)
			assert.Equal(t, once, twice)
		})
	}
}
