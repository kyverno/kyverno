package variables

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------
// Substitution safety tests, for lazy checkpoint materialization.
//
// The engine context's lazy Checkpoint() (pkg/engine/context) leaves the
// live jsonRaw graph aliasable through Query()/QueryOperation() results
// until a write forces materialization. That is only safe if no caller
// installs one of those results into a document it then mutates in
// place. On the real admission and background paths, every consumer of
// a resolved variable goes through variables.SubstituteAll, which drives
// pkg/engine/jsonutils' TraverseJSON: TraverseJSON's own recursion
// (traverseJSON's post-action switch, datautils.CopyMap for maps and
// slices.Clone plus recursive traverseObject/traverseList for both)
// deep-copies whatever a leaf action returns before it is merged back
// into the substituted document — see
// pkg/engine/jsonutils/traverse_test.go's
// Test_TraverseJSONCopiesLeafActionContainerResults, which pins that
// property directly.
//
// This means a resolved variable that happens to be a live map/slice
// (for example the whole-leaf case `"{{ request.object.metadata }}"`,
// where DefaultVariableResolver just returns ctx.Query's result) can
// never end up shared with the live context once it has passed through
// SubstituteAll: TraverseJSON isolates it regardless. No copy needs to
// be added in pkg/engine/variables itself — it would be redundant with
// TraverseJSON's existing copy and would tax every variable-using policy
// for nothing.
//
// The residual invariant is narrower than "no caller mutates a Query
// result": it is "no caller feeds a raw Query()/QueryOperation() result
// into an in-place mutator without going through SubstituteAll first."
// The direct call sites outside this package are audited read-only:
// pkg/engine/utils/foreach.go (EvaluateList, elements re-enter only via
// AddElement), pkg/engine/handlers/validation/validate_manifest.go
// (immediately json.Marshal'd), the "target" existence probe in this
// package (result discarded, see the `ctx.Query("target")` call a few
// lines above the whole-leaf substitution case), and the CLI's
// cmd/cli/kubectl-kyverno/processor/policy_processor.go:858-903 (each
// Query result is wrapped into a new unstructured.Unstructured and set
// via WithNewResource/WithOldResource, not mutated in place). A future
// direct-Query caller that mutates its result in place, bypassing
// SubstituteAll, would violate this invariant — that is a
// stop-and-escalate finding for review, not something silently patched
// here.
// -----------------------------------------------------------------------

func podResourceRaw() []byte {
	return []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-a",
			"namespace": "default",
			"labels": {
				"app": "x",
				"num": 2
			}
		}
	}
	`)
}

// Test_SubstitutionIndependence_MutatingResultDoesNotAffectLiveContext
// is the violator-catching guard: substituting a pattern that resolves
// to an object subtree, then mutating the returned value, must never be
// visible through a subsequent Query() of the same path. It passes today
// because TraverseJSON copies the substituted subtree (see the package
// comment above); it would fail if that copy were ever removed or
// narrowed, and it also would have failed if any code path fed the raw,
// uncopied Query() result to a mutator (which no current code does).
func Test_SubstitutionIndependence_MutatingResultDoesNotAffectLiveContext(t *testing.T) {
	ctx := context.NewContext(jp)
	assert.NoError(t, context.AddResource(ctx, podResourceRaw()))

	pattern := map[string]interface{}{
		"metadata": "{{ request.object.metadata }}",
	}

	result, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)

	metadata, ok := resultMap["metadata"].(map[string]interface{})
	assert.True(t, ok, "substituted metadata must be a map[string]interface{}")

	// Mutate the substituted value: add/rewrite a label, exactly the
	// shape of wildcards.ExpandInMetadata's `metadata[labelsKey] = labels`.
	labels, ok := metadata["labels"].(map[string]interface{})
	assert.True(t, ok)
	labels["num"] = float64(99)
	labels["injected"] = "boom"

	// The live context must be completely unaffected by the mutation
	// above.
	liveNum, err := ctx.Query("request.object.metadata.labels.num")
	assert.NoError(t, err)
	assert.Equal(t, float64(2), liveNum)

	_, err = ctx.Query("request.object.metadata.labels.injected")
	assert.Error(t, err, "mutation of the substituted document must not leak into the live context")
}

// Test_LazyProductionPath_SubstituteExpandValidateRestore is the
// production-path integration test for lazy checkpoint materialization:
// it reproduces the exact real-world call sequence
// (Checkpoint -> variables.SubstituteAll -> wildcards.ExpandInMetadata
// + validate.MatchPattern -> a write -> Restore) that a validate rule
// with a "{{ request.object.metadata }}" pattern performs, and asserts
// the restored context is byte-identical to the pre-mutation state —
// i.e. lazy behaves exactly like eager here. Unlike a synthetic
// raw-Query() repro (which bypasses SubstituteAll and reproduces nothing
// any production code path executes), this test only exercises entry
// points real rules call, so it is the correctness gate for reinstating
// lazy on the admission/background paths that use pattern substitution.
func Test_LazyProductionPath_SubstituteExpandValidateRestore(t *testing.T) {
	ctx := context.NewContext(jp)
	assert.NoError(t, context.AddResource(ctx, podResourceRaw()))

	ctx.Checkpoint()

	pattern := map[string]interface{}{
		"metadata": "{{ request.object.metadata }}",
	}
	substituted, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NoError(t, err)

	patternMap, ok := substituted.(map[string]interface{})
	assert.True(t, ok)

	resourceMap, err := ctx.Query("request.object")
	assert.NoError(t, err)
	resourceMapTyped, ok := resourceMap.(map[string]interface{})
	assert.True(t, ok)

	// ExpandInMetadata mutates patternMap["metadata"] in place (it
	// assigns metadata[labelsKey] = labels) whenever it finds a "labels"
	// or "annotations" key to expand wildcards in. Because patternMap
	// came from SubstituteAll, this can only ever touch TraverseJSON's
	// independent copy, never the live context.
	patternMap = wildcards.ExpandInMetadata(patternMap, resourceMapTyped)

	// validate.MatchPattern is the real consumer of a substituted
	// validation pattern (pkg/engine/handlers/validation/validate_resource.go
	// calls it with v.pattern after substitutePatterns()). It only reads
	// patternMap and resourceMapTyped; it does not mutate the live
	// context either.
	_ = validate.MatchPattern(logr.Discard(), resourceMapTyped, patternMap)

	// A write, as a real rule handler would perform.
	assert.NoError(t, ctx.AddVariable("someVar", "someValue"))

	ctx.Restore()

	num, err := ctx.Query("request.object.metadata.labels.num")
	assert.NoError(t, err)
	assert.Equal(t, float64(2), num, "Restore() must return the pre-mutation state (lazy must equal eager on the real admission path)")

	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}
