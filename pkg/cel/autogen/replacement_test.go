package autogen

import (
	"encoding/json"
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
			name:   "object metadata.?namespace (optional select) is preserved",
			expr:   "object.metadata.?namespace.orValue('')",
			config: "deployments",
			want:   "object.metadata.?namespace.orValue('')",
		},
		{
			name:   "cronjobs object metadata.?namespace (optional select) is preserved",
			expr:   "object.metadata.?namespace.orValue('')",
			config: "cronjobs",
			want:   "object.metadata.?namespace.orValue('')",
		},
		{
			name:   "oldObject metadata.?namespace (optional select) is preserved",
			expr:   "oldObject.metadata.?namespace",
			config: "deployments",
			want:   "oldObject.metadata.?namespace",
		},
		{
			name:   "namespace membership with optional select is preserved",
			expr:   "!(object.metadata.?namespace.orValue('') in ['kube-system'])",
			config: "deployments",
			want:   "!(object.metadata.?namespace.orValue('') in ['kube-system'])",
		},
		{
			name:   "bracket access metadata['namespace'] is preserved",
			expr:   "object.metadata['namespace'] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata['namespace'] == 'kube-system'",
		},
		{
			name:   "bracket access with double quotes is preserved",
			expr:   "object.metadata[\"namespace\"] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata[\"namespace\"] == 'kube-system'",
		},
		{
			name:   "bracket access with double quotes is preserved in JSON-marshaled bytes",
			expr:   "object.metadata[\\\"namespace\\\"] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata[\\\"namespace\\\"] == 'kube-system'",
		},
		{
			name:   "bracket access with triple single quotes is preserved",
			expr:   "object.metadata['''namespace'''] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata['''namespace'''] == 'kube-system'",
		},
		{
			name:   "bracket access with triple double quotes is preserved in JSON-marshaled bytes",
			expr:   "object.metadata[\\\"\\\"\\\"namespace\\\"\\\"\\\"] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata[\\\"\\\"\\\"namespace\\\"\\\"\\\"] == 'kube-system'",
		},
		{
			name:   "bracket access with a raw string is preserved",
			expr:   "object.metadata[r'namespace'] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata[r'namespace'] == 'kube-system'",
		},
		{
			name:   "bracket access with a raw double-quoted string is preserved in JSON-marshaled bytes",
			expr:   "object.metadata[r\\\"namespace\\\"] == 'kube-system'",
			config: "deployments",
			want:   "object.metadata[r\\\"namespace\\\"] == 'kube-system'",
		},
		{
			name:   "optional select of a longer identifier is still rewritten",
			expr:   "object.metadata.?namespaceFoo",
			config: "deployments",
			want:   "object.spec.template.metadata.?namespaceFoo",
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
			name:   "deployments object metadata.?namespace is preserved (optional select)",
			expr:   "object.metadata.?namespace.orValue('') in ['kube-system', 'kyverno']",
			config: "deployments",
			want:   "object.metadata.?namespace.orValue('') in ['kube-system', 'kyverno']",
		},
		{
			name:   "optional select of a longer identifier is still rewritten",
			expr:   "object.metadata.?namespaceFoo == 'foo'",
			config: "deployments",
			want:   "object.spec.template.metadata.?namespaceFoo == 'foo'",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyCEL([]byte(tt.expr), ReplacementsMap[ConfigsMap[tt.config].ReplacementsRef]...)
			assert.Equal(t, []byte(tt.want), got)
		})
	}
}

// TestProtectedFieldSpellings runs CEL spellings of an access to
// metadata.namespace and metadata.name through the path vpol and ivpol
// autogen use (the expression is JSON-marshaled with the policy spec,
// rewritten with Apply and unmarshaled again), and through ApplyCEL. The
// spellings are written out the way a policy author would type them instead
// of being derived from protectedSuffixes.
func TestProtectedFieldSpellings(t *testing.T) {
	preserved := []string{
		"object.metadata.namespace",
		"object.metadata.?namespace.orValue('')",
		"object.metadata['namespace']",
		`object.metadata["namespace"]`,
		"object.metadata['''namespace''']",
		`object.metadata["""namespace"""]`,
		"object.metadata[r'namespace']",
		"object.metadata[R'namespace']",
		`object.metadata[r"namespace"]`,
		`object.metadata[R"namespace"]`,
		"object.metadata[r'''namespace''']",
		`object.metadata[R"""namespace"""]`,
		"object.metadata[?'namespace'].orValue('')",
		`object.metadata[?"namespace"].orValue('')`,
		"object.metadata[?R'''namespace'''].orValue('')",
		"oldObject.metadata[?'namespace'].orValue('')",
		"object.metadata.name",
		"object.metadata.?name.orValue('')",
		"object.metadata['name']",
		`object.metadata["name"]`,
		"object.metadata[?'name'].orValue('')",
		`object.metadata[?r"name"].orValue('')`,
		"object.metadata[R'''name''']",
	}
	rewritten := []struct{ expr, want string }{
		{"object.metadata.?namespaceFoo", "object.spec.template.metadata.?namespaceFoo"},
		{"object.metadata[?'nameFoo']", "object.spec.template.metadata[?'nameFoo']"},
		{"object.metadata.labels['app']", "object.spec.template.metadata.labels['app']"},
		{"object.metadata[?'labels']", "object.spec.template.metadata[?'labels']"},
	}
	marshaled := func(t *testing.T, expr string, replacements []Replacement) string {
		t.Helper()
		data, err := json.Marshal(map[string]string{"expression": expr})
		assert.NoError(t, err)
		var out map[string]string
		assert.NoError(t, json.Unmarshal(Apply(data, replacements...), &out))
		return out["expression"]
	}
	for _, config := range []string{"deployments", "cronjobs"} {
		replacements := ReplacementsMap[ConfigsMap[config].ReplacementsRef]
		for _, expr := range preserved {
			t.Run(config+"/"+expr, func(t *testing.T) {
				assert.Equal(t, expr, marshaled(t, expr, replacements), "Apply on the marshaled spec")
				assert.Equal(t, expr, string(ApplyCEL([]byte(expr), replacements...)), "ApplyCEL")
			})
		}
	}
	replacements := ReplacementsMap[ConfigsMap["deployments"].ReplacementsRef]
	for _, tt := range rewritten {
		t.Run("deployments/"+tt.expr, func(t *testing.T) {
			assert.Equal(t, tt.want, marshaled(t, tt.expr, replacements), "Apply on the marshaled spec")
			assert.Equal(t, tt.want, string(ApplyCEL([]byte(tt.expr), replacements...)), "ApplyCEL")
		})
	}
}
