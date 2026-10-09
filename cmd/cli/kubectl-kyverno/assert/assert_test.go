package assert

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAssert(t *testing.T) {
	value := map[string]any{
		"name":     "check-team",
		"ruleType": "Validation",
		"message":  "The label `team` is required.",
		"status":   "fail",
		"items":    []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}},
		"labels":   map[string]any{"app": "myapp", "team": "platform"},
		"replicas": int64(2),
	}
	tests := []struct {
		name      string
		assertion string
		wantErrs  int
		wantErr   string
	}{
		{name: "fields", assertion: "{status: fail, message: 'The label `team` is required.'}"},
		{name: "field mismatch", assertion: "{status: pass}", wantErrs: 1},
		{name: "missing field", assertion: "{missing: foo}", wantErrs: 1},
		{name: "nested", assertion: "{labels: {team: platform}}"},
		{name: "number", assertion: "{replicas: 2}"},
		{name: "list", assertion: "{items: [{name: a}, {name: b}]}"},
		{name: "list length mismatch", assertion: "{items: [{name: a}]}", wantErrs: 1},
		{name: "expression key", assertion: "{(object.message.contains('`team`')): true}"},
		{name: "expression key mismatch", assertion: "{(object.status == 'pass'): true}", wantErrs: 1},
		{name: "expression projection", assertion: "{(size(object.items)): 2}"},
		{name: "expression value", assertion: "{replicas: (1 + 1)}"},
		{name: "expression value mismatch", assertion: "{replicas: (1 + 2)}", wantErrs: 1},
		{name: "foreach", assertion: "{~.(object.items): {(size(object.name)): 1}}"},
		{name: "foreach mismatch", assertion: "{~.items: {name: a}}", wantErrs: 1},
		{name: "foreach map with name", assertion: "{~key.labels: {(size(bindings.key) > 2): true}}"},
		{name: "binding", assertion: "{(object.labels) -> labels: {(bindings.labels.team): platform}}"},
		{name: "escape", assertion: `{\(status)\: foo}`, wantErrs: 1},
		{name: "jmespath", assertion: "{(length(message)): 29}", wantErr: "expressions must be CEL, JMESPath is no longer supported"},
		{name: "not comparable", assertion: "{status: 1}", wantErr: "types are not comparable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var assertion any
			require.NoError(t, yaml.Unmarshal([]byte(tt.assertion), &assertion))
			errs, err := Assert(context.Background(), nil, Parse(context.Background(), assertion), value, nil)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, errs, tt.wantErrs, errs)
		})
	}
}
