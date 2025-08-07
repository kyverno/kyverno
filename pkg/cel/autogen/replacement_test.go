package autogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateGenRuleByte(t *testing.T) {
	tests := []struct {
		pbyte  []byte
		config string
		want   []byte
	}{
		{
			pbyte:  []byte("object.spec"),
			config: "deployments",
			want:   []byte("object.spec.template.spec"),
		},
		{
			pbyte:  []byte("oldObject.spec"),
			config: "deployments",
			want:   []byte("oldObject.spec.template.spec"),
		},
		{
			pbyte:  []byte("object.spec"),
			config: "cronjobs",
			want:   []byte("object.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte:  []byte("oldObject.spec"),
			config: "cronjobs",
			want:   []byte("oldObject.spec.jobTemplate.spec.template.spec"),
		},
		{
			pbyte:  []byte("object.metadata"),
			config: "deployments",
			want:   []byte("object.spec.template.metadata"),
		},
		{
			pbyte:  []byte("object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
			config: "cronjobs",
			want:   []byte("object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
		},
		{
			pbyte:  []byte("object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
			config: "deployments",
			want:   []byte("object.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)"),
		},
	}
	for _, tt := range tests {
		got := Apply(tt.pbyte, ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
		assert.Equal(t, tt.want, got)
	}
}
