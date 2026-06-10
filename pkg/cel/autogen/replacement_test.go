package autogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateGenRuleByte(t *testing.T) {
	tests := []struct {
		name   string
		pbyte  []byte
		config string
		want   []byte
	}{
		{
			name:   "deployments spec",
			pbyte:  []byte("object.spec"),
			config: "deployments",
			want:   []byte("object.spec.template.spec"),
		},
		{
			name:   "deployments oldObject spec",
			pbyte:  []byte("oldObject.spec"),
			config: "deployments",
			want:   []byte("oldObject.spec.template.spec"),
		},
		{
			name:   "cronjobs spec",
			pbyte:  []byte("object.spec"),
			config: "cronjobs",
			want:   []byte("object.spec.jobTemplate.spec.template.spec"),
		},
		{
			name:   "cronjobs oldObject spec",
			pbyte:  []byte("oldObject.spec"),
			config: "cronjobs",
			want:   []byte("oldObject.spec.jobTemplate.spec.template.spec"),
		},
		{
			name:   "deployments metadata is rewritten",
			pbyte:  []byte("object.metadata"),
			config: "deployments",
			want:   []byte("object.spec.template.metadata"),
		},
		{
			name:   "deployments metadata.labels is rewritten",
			pbyte:  []byte("object.metadata.labels['app']"),
			config: "deployments",
			want:   []byte("object.spec.template.metadata.labels['app']"),
		},
		{
			name:   "deployments object metadata.namespace is preserved",
			pbyte:  []byte("object.metadata.namespace"),
			config: "deployments",
			want:   []byte("object.metadata.namespace"),
		},
		{
			name:   "deployments oldObject metadata.namespace is preserved",
			pbyte:  []byte("oldObject.metadata.namespace"),
			config: "deployments",
			want:   []byte("oldObject.metadata.namespace"),
		},
		{
			name:   "cronjobs object metadata.namespace is preserved",
			pbyte:  []byte("object.metadata.namespace"),
			config: "cronjobs",
			want:   []byte("object.metadata.namespace"),
		},
		{
			name:   "cronjobs oldObject metadata.namespace is preserved",
			pbyte:  []byte("oldObject.metadata.namespace"),
			config: "cronjobs",
			want:   []byte("oldObject.metadata.namespace"),
		},
		{
			name:   "namespace membership expression is preserved (deployments)",
			pbyte:  []byte("!(object.metadata.namespace in ['opencost', 'kube-system'])"),
			config: "deployments",
			want:   []byte("!(object.metadata.namespace in ['opencost', 'kube-system'])"),
		},
		{
			name:   "namespace membership expression is preserved (cronjobs)",
			pbyte:  []byte("!(object.metadata.namespace in ['opencost', 'kube-system'])"),
			config: "cronjobs",
			want:   []byte("!(object.metadata.namespace in ['opencost', 'kube-system'])"),
		},
		{
			name:   "namespace preserved while sibling metadata fields are rewritten",
			pbyte:  []byte("object.metadata.namespace == 'foo' && object.metadata.labels['team'] == 'platform'"),
			config: "deployments",
			want:   []byte("object.metadata.namespace == 'foo' && object.spec.template.metadata.labels['team'] == 'platform'"),
		},
		{
			name:   "only the namespace segment is protected, not longer identifiers",
			pbyte:  []byte("object.metadata.namespaceFoo"),
			config: "deployments",
			want:   []byte("object.spec.template.metadata.namespaceFoo"),
		},
		{
			name:   "user content containing protected sentinel-like text is not corrupted",
			pbyte:  []byte("object.metadata.labels['__KYVERNO_PROTECTED_OBJECT_METADATA_NAMESPACE__'] == 'x'"),
			config: "deployments",
			want:   []byte("object.spec.template.metadata.labels['__KYVERNO_PROTECTED_OBJECT_METADATA_NAMESPACE__'] == 'x'"),
		},
		{
			name:   "cronjobs containers expression",
			pbyte:  []byte("object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
			config: "cronjobs",
			want:   []byte("object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
		},
		{
			name:   "deployments containers expression",
			pbyte:  []byte("object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
			config: "deployments",
			want:   []byte("object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply(tt.pbyte, ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
			assert.Equal(t, string(tt.want), string(got))
		})
	}
}
