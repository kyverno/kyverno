package validate

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"gotest.tools/v3/assert"
)

func TestSubstituteAndMatchPatternDoesNotMutateContext(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	ctx := context.NewContext(jmespath.New(cfg))
	pod := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "pod",
			"labels": map[string]interface{}{
				"app": "nginx",
			},
		},
	}
	assert.NilError(t, context.AddResource(ctx, mustJSON(pod)))

	ctx.Checkpoint()
	pattern := map[string]interface{}{
		"metadata": "{{ request.object.metadata }}",
	}
	substituted, err := variables.SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	resource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "pod",
			"labels": map[string]interface{}{
				"app": "nginx",
			},
		},
	}
	err = MatchPattern(logr.Discard(), resource, substituted)
	assert.NilError(t, err)
	ctx.Restore()

	labels, err := ctx.Query("request.object.metadata.labels")
	assert.NilError(t, err)
	labelMap, ok := labels.(map[string]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, "nginx", labelMap["app"])
	_, hasExpanded := labelMap["*=nginx"]
	assert.Assert(t, !hasExpanded)
}

func mustJSON(v map[string]interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
