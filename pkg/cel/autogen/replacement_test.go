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
			name:   "deployments object metadata.name is preserved",
			expr:   "object.metadata.name",
			config: "deployments",
			want:   "object.metadata.name",
		},
		{
			name:   "deployments oldObject metadata.name is preserved",
			expr:   "oldObject.metadata.name",
			config: "deployments",
			want:   "oldObject.metadata.name",
		},
		{
			name:   "cronjobs object metadata.name is preserved",
			expr:   "object.metadata.name",
			config: "cronjobs",
			want:   "object.metadata.name",
		},
		{
			name:   "cronjobs oldObject metadata.name is preserved",
			expr:   "oldObject.metadata.name",
			config: "cronjobs",
			want:   "oldObject.metadata.name",
		},
		{
			name:   "name preserved while sibling metadata fields are rewritten",
			expr:   "object.metadata.name + ' in ' + object.metadata.labels['team']",
			config: "deployments",
			want:   "object.metadata.name + ' in ' + object.spec.template.metadata.labels['team']",
		},
		{
			name:   "only the name segment is protected, not longer identifiers",
			expr:   "object.metadata.nameFoo",
			config: "deployments",
			want:   "object.spec.template.metadata.nameFoo",
		},
		{
			name:   "JSON-escaped indexed metadata.name is preserved",
			expr:   "object.metadata[\\\"name\\\"]",
			config: "deployments",
			want:   "object.metadata[\\\"name\\\"]",
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
			name:   "deployments object.?spec is rewritten",
			expr:   "object.?spec",
			config: "deployments",
			want:   "object.?spec.template.spec",
		},
		{
			name:   "deployments oldObject.?spec is rewritten",
			expr:   "oldObject.?spec",
			config: "deployments",
			want:   "oldObject.?spec.template.spec",
		},
		{
			name:   "cronjobs object.?spec is rewritten",
			expr:   "object.?spec",
			config: "cronjobs",
			want:   "object.?spec.jobTemplate.spec.template.spec",
		},
		{
			name:   "cronjobs oldObject.?spec is rewritten",
			expr:   "oldObject.?spec",
			config: "cronjobs",
			want:   "oldObject.?spec.jobTemplate.spec.template.spec",
		},
		{
			name:   "deployments object.?metadata.?labels is rewritten",
			expr:   "object.?metadata.?labels['app']",
			config: "deployments",
			want:   "object.?spec.template.metadata.?labels['app']",
		},
		{
			name:   "deployments object.?metadata.labels is rewritten",
			expr:   "object.?metadata.labels['app']",
			config: "deployments",
			want:   "object.?spec.template.metadata.labels['app']",
		},
		{
			name:   "deployments object.metadata.?labels is rewritten",
			expr:   "object.metadata.?labels['app']",
			config: "deployments",
			want:   "object.spec.template.metadata.?labels['app']",
		},
		{
			name:   "deployments object.?metadata.?name is preserved",
			expr:   "object.?metadata.?name",
			config: "deployments",
			want:   "object.?metadata.?name",
		},
		{
			name:   "deployments oldObject.?metadata.?name is preserved",
			expr:   "oldObject.?metadata.?name",
			config: "deployments",
			want:   "oldObject.?metadata.?name",
		},
		{
			name:   "deployments object.?metadata.?namespace is preserved",
			expr:   "object.?metadata.?namespace",
			config: "deployments",
			want:   "object.?metadata.?namespace",
		},
		{
			name:   "deployments oldObject.?metadata.?namespace is preserved",
			expr:   "oldObject.?metadata.?namespace",
			config: "deployments",
			want:   "oldObject.?metadata.?namespace",
		},
		{
			name:   "cronjobs object.?metadata.?name is preserved",
			expr:   "object.?metadata.?name",
			config: "cronjobs",
			want:   "object.?metadata.?name",
		},
		{
			name:   "cronjobs object.?metadata.?namespace is preserved",
			expr:   "object.?metadata.?namespace",
			config: "cronjobs",
			want:   "object.?metadata.?namespace",
		},
		{
			name:   "optional traversal with containers expression",
			expr:   "object.?spec.?containers.orValue([]).all(c, !c.image.endsWith(':latest'))",
			config: "deployments",
			want:   "object.?spec.template.spec.?containers.orValue([]).all(c, !c.image.endsWith(':latest'))",
		},
		{
			name:   "deployments object['spec'] is rewritten",
			expr:   "object['spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[\"spec\"] is rewritten",
			expr:   "object[\"spec\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments JSON-escaped object[\\\"spec\\\"] is rewritten",
			expr:   "object[\\\"spec\\\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[?'spec'] is rewritten",
			expr:   "object[?'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[?'spec'].?containers is rewritten",
			expr:   "object[?'spec'].?containers.orValue([]).all(c, !c.image.endsWith(':latest'))",
			config: "deployments",
			want:   "object.?spec.template.spec.?containers.orValue([]).all(c, !c.image.endsWith(':latest'))",
		},
		{
			name:   "deployments object[?\"spec\"] is rewritten",
			expr:   "object[?\"spec\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments JSON-escaped object[?\\\"spec\\\"] is rewritten",
			expr:   "object[?\\\"spec\\\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments oldObject[?'spec'] is rewritten",
			expr:   "oldObject[?'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "oldObject.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "cronjobs object[?'spec'] is rewritten",
			expr:   "object[?'spec'].containers.exists(c, c.name == 'nginx')",
			config: "cronjobs",
			want:   "object.?spec.jobTemplate.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[?'metadata'][?'name'] is preserved",
			expr:   "object[?'metadata'][?'name'] == 'foo'",
			config: "deployments",
			want:   "object[?'metadata'][?'name'] == 'foo'",
		},
		{
			name:   "deployments object[?'metadata'][?'namespace'] is preserved",
			expr:   "object[?'metadata'][?'namespace'] == 'foo'",
			config: "deployments",
			want:   "object[?'metadata'][?'namespace'] == 'foo'",
		},
		{
			name:   "deployments object.?metadata[?'name'] is preserved",
			expr:   "object.?metadata[?'name'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?'name'] == 'foo'",
		},
		{
			name:   "deployments object.?metadata[?'namespace'] is preserved",
			expr:   "object.?metadata[?'namespace'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?'namespace'] == 'foo'",
		},
		{
			name:   "deployments object.?metadata[?'labels'] is rewritten",
			expr:   "object.?metadata[?'labels']['env'] == 'prod'",
			config: "deployments",
			want:   "object.?spec.template.metadata[?'labels']['env'] == 'prod'",
		},
		{
			name:   "deployments foo.?object.?spec is skipped",
			expr:   "foo.?object.?spec == true",
			config: "deployments",
			want:   "foo.?object.?spec == true",
		},
		{
			name:   "deployments request.?object.?spec is rewritten",
			expr:   "request.?object.?spec.replicas > 0",
			config: "deployments",
			want:   "request.?object.?spec.template.spec.replicas > 0",
		},
		{
			name:   "deployments object.spec.templateSpec is rewritten",
			expr:   "object.spec.templateSpec == 'foo'",
			config: "deployments",
			want:   "object.spec.template.spec.templateSpec == 'foo'",
		},
		{
			name:   "deployments object.spec.jobTemplateX is rewritten",
			expr:   "object.spec.jobTemplateX == 'foo'",
			config: "deployments",
			want:   "object.spec.template.spec.jobTemplateX == 'foo'",
		},
		{
			name:   "deployments object[? 'spec'] with whitespace is rewritten",
			expr:   "object[? 'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object.?metadata[? 'name'] with whitespace is preserved",
			expr:   "object.?metadata[? 'name'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[? 'name'] == 'foo'",
		},
		{
			name:   "deployments object.?metadata[? 'namespace'] with whitespace is preserved",
			expr:   "object.?metadata[? 'namespace'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[? 'namespace'] == 'foo'",
		},
		{
			name:   "deployments object.metadata.labels['name'] is rewritten",
			expr:   "object.metadata.labels['name'] == 'foo'",
			config: "deployments",
			want:   "object.spec.template.metadata.labels['name'] == 'foo'",
		},
		{
			name:   "deployments object.metadata.labels.name is rewritten",
			expr:   "object.metadata.labels.name == 'foo'",
			config: "deployments",
			want:   "object.spec.template.metadata.labels.name == 'foo'",
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
			name:   "metadata.name is preserved (single-quote bracket syntax)",
			expr:   "object.metadata['name'] == 'foo'",
			config: "deployments",
			want:   "object.metadata['name'] == 'foo'",
		},
		{
			name:   "metadata.name is preserved (double-quote bracket syntax)",
			expr:   "object.metadata[\"name\"] == 'foo'",
			config: "deployments",
			want:   "object.metadata[\"name\"] == 'foo'",
		},
		{
			name:   "string literals are not rewritten",
			expr:   "object.metadata.name == 'object.spec'",
			config: "deployments",
			want:   "object.metadata.name == 'object.spec'",
		},
		{
			name:   "triple quoted string literals are not rewritten",
			expr:   "object.metadata.name == '''object.spec'''",
			config: "deployments",
			want:   "object.metadata.name == '''object.spec'''",
		},
		{
			name:   "triple double quoted string literals are not rewritten",
			expr:   "object.metadata.name == \"\"\"object.spec\"\"\"",
			config: "deployments",
			want:   "object.metadata.name == \"\"\"object.spec\"\"\"",
		},
		{
			name:   "raw single quoted string literals are not rewritten",
			expr:   "object.metadata.name == r'object.spec'",
			config: "deployments",
			want:   "object.metadata.name == r'object.spec'",
		},
		{
			name:   "backtick string literals are not rewritten",
			expr:   "object.metadata.name == `object.spec`",
			config: "deployments",
			want:   "object.metadata.name == `object.spec`",
		},
		{
			name:   "escaped string literals are handled correctly",
			expr:   "object.metadata.name == 'object\\'spec' && object.spec.replicas > 0",
			config: "deployments",
			want:   "object.metadata.name == 'object\\'spec' && object.spec.template.spec.replicas > 0",
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
		{
			name:   "optional selector forms (single quote bracket) are rewritten",
			expr:   "object[?'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "optional selector forms (double quote bracket) are rewritten",
			expr:   "object[?\"spec\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "optional selector forms oldObject (single quote bracket) are rewritten",
			expr:   "oldObject[?'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "oldObject.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "optional selector forms oldObject (double quote bracket) are rewritten",
			expr:   "oldObject[?\"spec\"].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "oldObject.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "object.?metadata[?'namespace'] is preserved",
			expr:   "object.?metadata[?'namespace'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?'namespace'] == 'foo'",
		},
		{
			name:   "object.?metadata[?\"namespace\"] is preserved",
			expr:   "object.?metadata[?\"namespace\"] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?\"namespace\"] == 'foo'",
		},
		{
			name:   "object.?metadata[?'name'] is preserved",
			expr:   "object.?metadata[?'name'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?'name'] == 'foo'",
		},
		{
			name:   "object.?metadata[?\"name\"] is preserved",
			expr:   "object.?metadata[?\"name\"] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?\"name\"] == 'foo'",
		},
		{
			name:   "JSON-escaped optional metadata.name is preserved",
			expr:   "object.?metadata[?\\\"name\\\"] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?\\\"name\\\"] == 'foo'",
		},
		{
			name:   "JSON-escaped optional metadata.namespace is preserved",
			expr:   "object.?metadata[?\\\"namespace\\\"] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[?\\\"namespace\\\"] == 'foo'",
		},
		{
			name:   "request.object.?spec is rewritten",
			expr:   "request.object.?spec.replicas > 0",
			config: "deployments",
			want:   "request.object.?spec.template.spec.replicas > 0",
		},
		{
			name:   "request.?object.?spec is rewritten",
			expr:   "request.?object.?spec.replicas > 0",
			config: "deployments",
			want:   "request.?object.?spec.template.spec.replicas > 0",
		},
		{
			name:   "request.?object.spec is rewritten",
			expr:   "request.?object.spec.replicas > 0",
			config: "deployments",
			want:   "request.?object.spec.template.spec.replicas > 0",
		},
		{
			name:   "foo.?object.?spec is skipped",
			expr:   "foo.?object.?spec == true",
			config: "deployments",
			want:   "foo.?object.?spec == true",
		},
		{
			name:   "string literals containing object.?spec are not rewritten",
			expr:   "object.metadata.name == 'object.?spec'",
			config: "deployments",
			want:   "object.metadata.name == 'object.?spec'",
		},
		{
			name:   "cronjobs object.?spec is rewritten",
			expr:   "object.?spec",
			config: "cronjobs",
			want:   "object.?spec.jobTemplate.spec.template.spec",
		},
		{
			name:   "cronjobs oldObject.?spec is rewritten",
			expr:   "oldObject.?spec",
			config: "cronjobs",
			want:   "oldObject.?spec.jobTemplate.spec.template.spec",
		},
		{
			name:   "cronjobs object[?'spec'] is rewritten",
			expr:   "object[?'spec'].containers.exists(c, c.name == 'nginx')",
			config: "cronjobs",
			want:   "object.?spec.jobTemplate.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object.?metadata.?labels is rewritten",
			expr:   "object.?metadata.?labels['app']",
			config: "deployments",
			want:   "object.?spec.template.metadata.?labels['app']",
		},
		{
			name:   "deployments object.?metadata.labels is rewritten",
			expr:   "object.?metadata.labels['app']",
			config: "deployments",
			want:   "object.?spec.template.metadata.labels['app']",
		},
		{
			name:   "deployments object.metadata.?labels is rewritten",
			expr:   "object.metadata.?labels['app']",
			config: "deployments",
			want:   "object.spec.template.metadata.?labels['app']",
		},
		{
			name:   "cronjobs object.?metadata.?labels is rewritten",
			expr:   "object.?metadata.?labels['app']",
			config: "cronjobs",
			want:   "object.?spec.jobTemplate.spec.template.metadata.?labels['app']",
		},
		{
			name:   "deployments object.spec.templateSpec is rewritten in CEL",
			expr:   "object.spec.templateSpec == 'foo'",
			config: "deployments",
			want:   "object.spec.template.spec.templateSpec == 'foo'",
		},
		{
			name:   "deployments object.spec.jobTemplateX is rewritten in CEL",
			expr:   "object.spec.jobTemplateX == 'foo'",
			config: "deployments",
			want:   "object.spec.template.spec.jobTemplateX == 'foo'",
		},
		{
			name:   "deployments object[? 'spec'] with whitespace is rewritten in CEL",
			expr:   "object[? 'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments comment with apostrophe does not prevent spec rewrite in CEL",
			expr:   "// don't match\nobject.spec.replicas > 0",
			config: "deployments",
			want:   "// don't match\nobject.spec.template.spec.replicas > 0",
		},
		{
			name:   "deployments raw triple-quoted string literal is not rewritten",
			expr:   "r\"\"\"a\" object.?spec.containers \"\"\"",
			config: "deployments",
			want:   "r\"\"\"a\" object.?spec.containers \"\"\"",
		},
		{
			name:   "deployments object.metadata.labels['name'] is rewritten in CEL",
			expr:   "object.metadata.labels['name'] == 'foo'",
			config: "deployments",
			want:   "object.spec.template.metadata.labels['name'] == 'foo'",
		},
		{
			name:   "deployments object.metadata.labels.name is rewritten in CEL",
			expr:   "object.metadata.labels.name == 'foo'",
			config: "deployments",
			want:   "object.spec.template.metadata.labels.name == 'foo'",
		},
		{
			name:   "deployments object.?metadata[? 'name'] with whitespace is preserved in CEL",
			expr:   "object.?metadata[? 'name'] == 'foo'",
			config: "deployments",
			want:   "object.?metadata[? 'name'] == 'foo'",
		},
		{
			name:   "deployments object.metadata[?'name' ] with whitespace before closing bracket is preserved in CEL",
			expr:   "object.metadata[?'name' ] == 'foo'",
			config: "deployments",
			want:   "object.metadata[?'name' ] == 'foo'",
		},
		{
			name:   "deployments object.metadata[? 'name' ] with whitespace on both sides is preserved in CEL",
			expr:   "object.metadata[? 'name' ] == 'foo'",
			config: "deployments",
			want:   "object.metadata[? 'name' ] == 'foo'",
		},
		{
			name:   "deployments object.metadata[ 'name' ] with spaces is preserved in CEL",
			expr:   "object.metadata[ 'name' ] == 'foo'",
			config: "deployments",
			want:   "object.metadata[ 'name' ] == 'foo'",
		},
		{
			name:   "deployments object.metadata[ ?\"namespace\" ] with spaces is preserved in CEL",
			expr:   "object.metadata[ ?\"namespace\" ] == 'default'",
			config: "deployments",
			want:   "object.metadata[ ?\"namespace\" ] == 'default'",
		},
		{
			name:   "deployments object.metadata[?\\\"name\\\" ] with JSON escaped quote and space is preserved in CEL",
			expr:   "object.metadata[?\\\"name\\\" ] == 'foo'",
			config: "deployments",
			want:   "object.metadata[?\\\"name\\\" ] == 'foo'",
		},
		{
			name:   "deployments object.metadata[?'other' ] is rewritten to template metadata in CEL",
			expr:   "object.metadata[?'other' ] == 'foo'",
			config: "deployments",
			want:   "object.spec.template.metadata[?'other' ] == 'foo'",
		},
		{
			name:   "deployments block comment with apostrophe does not prevent spec rewrite in CEL",
			expr:   "/* don't match */ object.spec.replicas > 0",
			config: "deployments",
			want:   "/* don't match */ object.spec.template.spec.replicas > 0",
		},
		{
			name:   "deployments object[?  'spec' ] with multiple internal spaces is rewritten in CEL",
			expr:   "object[?  'spec' ].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[ ? 'spec' ] with space before ? is rewritten in CEL",
			expr:   "object[ ? 'spec' ].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[\\n  'spec'\\n] with newlines is rewritten in CEL",
			expr:   "object[\n  'spec'\n].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object [? 'spec'] with space before bracket is rewritten in CEL",
			expr:   "object [? 'spec'].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "object.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments oldObject[?  'spec' ] is rewritten in CEL",
			expr:   "oldObject[?  'spec' ].containers.exists(c, c.name == 'nginx')",
			config: "deployments",
			want:   "oldObject.?spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:   "deployments object[?  'metadata' ][?  'namespace' ] is preserved in CEL",
			expr:   "object[?  'metadata' ][?  'namespace' ] == 'default'",
			config: "deployments",
			want:   "object[?  'metadata' ][?  'namespace' ] == 'default'",
		},
		{
			name:   "deployments object[?  'metadata' ][?  'name' ] is preserved in CEL",
			expr:   "object[?  'metadata' ][?  'name' ] == 'foo'",
			config: "deployments",
			want:   "object[?  'metadata' ][?  'name' ] == 'foo'",
		},
		{
			name:   "deployments object[?  'metadata' ][?  'labels' ] is rewritten in CEL",
			expr:   "object[?  'metadata' ][?  'labels' ]['env'] == 'prod'",
			config: "deployments",
			want:   "object.?spec.template.metadata[?  'labels' ]['env'] == 'prod'",
		},
		{
			name:   "spaced field select is skipped (foo . object . spec)",
			expr:   "foo . object . spec == true",
			config: "deployments",
			want:   "foo . object . spec == true",
		},
		{
			name:   "spaced optional field select is skipped (foo .? object . spec)",
			expr:   "foo .? object . spec == true",
			config: "deployments",
			want:   "foo .? object . spec == true",
		},
		{
			name:   "spaced optional field select is skipped (foo . ? object . spec)",
			expr:   "foo . ? object . spec == true",
			config: "deployments",
			want:   "foo . ? object . spec == true",
		},
		{
			name:   "spaced request.object is rewritten (request . object . spec)",
			expr:   "request . object . spec.replicas > 0",
			config: "deployments",
			want:   "request . object.spec.template.spec.replicas > 0",
		},
		{
			name:   "spaced request.?object is rewritten (request .? object .? spec)",
			expr:   "request .? object .? spec.replicas > 0",
			config: "deployments",
			want:   "request .? object.?spec.template.spec.replicas > 0",
		},
		{
			name:   "spaced myrequest.object is skipped (myrequest . object . spec)",
			expr:   "myrequest . object . spec.replicas > 0",
			config: "deployments",
			want:   "myrequest . object . spec.replicas > 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyCEL([]byte(tt.expr), ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
			assert.Equal(t, []byte(tt.want), got)
		})
	}
}

func TestApplyMutatingJSONPatch(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		config string
		want   string
	}{
		{
			name:   "spec path and expression are moved to the pod template",
			expr:   `[JSONPatch{op: "add", path: "/spec/tolerations", value: object.spec.tolerations}]`,
			config: "deployments",
			want:   `[JSONPatch{op: "add", path: "/spec/template/spec/tolerations", value: object.spec.template.spec.tolerations}]`,
		},
		{
			name:   "cronjob spec path is moved to the job template",
			expr:   `[JSONPatch{op: "add", path: "/spec/containers/0/imagePullPolicy", value: "Always"}]`,
			config: "cronjobs",
			want:   `[JSONPatch{op: "add", path: "/spec/jobTemplate/spec/template/spec/containers/0/imagePullPolicy", value: "Always"}]`,
		},
		{
			name:   "concatenated path keeps its dynamic suffix",
			expr:   `object.spec.containers.map(c, JSONPatch{op: "replace", path: "/spec/containers/" + string(object.spec.containers.indexOf(c)) + "/image", value: c.image})`,
			config: "deployments",
			want:   `object.spec.template.spec.containers.map(c, JSONPatch{op: "replace", path: "/spec/template/spec/containers/" + string(object.spec.template.spec.containers.indexOf(c)) + "/image", value: c.image})`,
		},
		{
			name:   "annotations and labels paths are moved to the template metadata",
			expr:   `[JSONPatch{op: "add", path: '/metadata/annotations', value: {}}, JSONPatch{op: "add", path: "/metadata/labels/app", value: "x"}]`,
			config: "deployments",
			want:   `[JSONPatch{op: "add", path: '/spec/template/metadata/annotations', value: {}}, JSONPatch{op: "add", path: "/spec/template/metadata/labels/app", value: "x"}]`,
		},
		{
			name:   "move from and path are both rewritten",
			expr:   `[JSONPatch{op: "move", from: "/spec/hostname", path: "/spec/subdomain"}]`,
			config: "deployments",
			want:   `[JSONPatch{op: "move", from: "/spec/template/spec/hostname", path: "/spec/template/spec/subdomain"}]`,
		},
		{
			name:   "controller name, namespace and similar prefixes are kept",
			expr:   `[JSONPatch{op: "test", path: "/metadata/name", value: "x"}, JSONPatch{op: "add", path: "/specs", value: "x"}, JSONPatch{op: "add", path: "/metadata/labelsX", value: "x"}]`,
			config: "deployments",
			want:   `[JSONPatch{op: "test", path: "/metadata/name", value: "x"}, JSONPatch{op: "add", path: "/specs", value: "x"}, JSONPatch{op: "add", path: "/metadata/labelsX", value: "x"}]`,
		},
		{
			name:   "pointer-like values and non-field strings are kept",
			expr:   `[JSONPatch{op: "add", path: "/spec/x", value: "/spec/y"}].filter(p, p.path != "/spec/z" && has(p.value) ? true : false)`,
			config: "deployments",
			want:   `[JSONPatch{op: "add", path: "/spec/template/spec/x", value: "/spec/y"}].filter(p, p.path != "/spec/z" && has(p.value) ? true : false)`,
		},
		{
			name:   "escaped quote inside the path is kept",
			expr:   `[JSONPatch{op: "add", path: "/metadata/annotations/a\"b", value: "x"}]`,
			config: "deployments",
			want:   `[JSONPatch{op: "add", path: "/spec/template/metadata/annotations/a\"b", value: "x"}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(ApplyMutatingJSONPatch([]byte(tt.expr), tt.config)))
		})
	}
}
