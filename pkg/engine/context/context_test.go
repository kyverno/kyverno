package context

import (
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	jp  = jmespath.New(config.NewDefaultConfiguration(false))
	cfg = config.NewDefaultConfiguration(false)
)

func Test_addResourceAndUserContext(t *testing.T) {
	var err error
	rawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "image-with-hostpath",
		   "labels": {
			  "app.type": "prod",
			  "namespace": "my-namespace"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "image-with-hostpath",
				 "image": "docker.io/nautiker/curl",
				 "volumeMounts": [
					{
					   "name": "var-lib-etcd",
					   "mountPath": "/var/lib"
					}
				 ]
			  }
		   ],
		   "volumes": [
			  {
				 "name": "var-lib-etcd",
				 "emptyDir": {}
			  }
		   ]
		}
	 }
			`)

	userInfo := authenticationv1.UserInfo{
		Username: "system:serviceaccount:nirmata:user1",
		UID:      "014fbff9a07c",
	}
	userRequestInfo := urkyverno.RequestInfo{
		Roles:             nil,
		ClusterRoles:      nil,
		AdmissionUserInfo: userInfo,
	}

	var expectedResult string
	ctx := NewContext(jp)
	err = AddResource(ctx, rawResource)
	if err != nil {
		t.Error(err)
	}
	result, err := ctx.Query("request.object.apiVersion")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "v1"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}

	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		t.Error(err)
	}
	result, err = ctx.Query("request.object.apiVersion")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "v1"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}

	result, err = ctx.Query("request.userInfo.username")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "system:serviceaccount:nirmata:user1"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}
	// Add service account Name
	err = ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		t.Error(err)
	}
	result, err = ctx.Query("serviceAccountName")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "user1"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}

	// Add service account Namespace
	result, err = ctx.Query("serviceAccountNamespace")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "nirmata"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("expected result does not match")
	}
}

func TestAddVariable(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        interface{}
		wantErr      bool
		query        string
		expected     interface{}
		wantQueryErr bool
	}{{
		name:         "Simple variable",
		key:          "simpleKey",
		value:        "simpleValue",
		wantErr:      false,
		wantQueryErr: false,
		expected:     "simpleValue",
	}, {
		name:         "Nested variable",
		key:          "nested.key",
		value:        123,
		wantErr:      false,
		wantQueryErr: false,
		expected:     123,
	}, {
		name:         "Invalid key format",
		key:          "invalid,key",
		value:        "someValue",
		wantErr:      false,
		wantQueryErr: true,
		expected:     nil,
	}, {
		name:         "Complex nested variable",
		key:          "complex.nested.key",
		value:        map[string]interface{}{"innerKey": "innerValue"},
		wantErr:      false,
		wantQueryErr: false,
		expected:     map[string]interface{}{"innerKey": "innerValue"},
	}, {
		name:         "Array value",
		key:          "arrayKey",
		value:        []int{1, 2, 3},
		wantErr:      false,
		wantQueryErr: false,
		expected:     []int{1, 2, 3},
	}, {
		name:         "Boolean value",
		key:          "boolKey",
		value:        true,
		wantErr:      false,
		wantQueryErr: false,
		expected:     true,
	}, {
		name:         "Empty key",
		key:          "",
		value:        "someValue",
		wantErr:      true,
		wantQueryErr: false,
		expected:     nil,
	}, {
		name:         "Nil value",
		key:          "nilKey",
		value:        nil,
		wantErr:      false,
		wantQueryErr: false,
		expected:     nil,
	}, {
		name:    "Escaped complex key",
		key:     `metadata.labels."com.example/my-label"`,
		value:   "foo",
		wantErr: false,
		query:   "metadata",
		expected: map[string]any{
			"labels": map[string]any{
				"com.example/my-label": "foo",
			},
		},
		wantQueryErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.NewDefaultConfiguration(false)
			jp := jmespath.New(conf)
			ctx := NewContext(jp)
			err := ctx.AddVariable(tt.key, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				query := tt.query
				if query == "" {
					query = tt.key
				}
				result, queryErr := ctx.Query(query)
				if tt.wantQueryErr {
					assert.Error(t, queryErr)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				}
			}
		})
	}
}

func Test_ImageInfoLoader(t *testing.T) {
	resource1, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "name": "test-pod",
		  "namespace": "default"
		},
		"spec": {
		  "containers": [{
			"name": "test_container",
			"image": "nginx:latest"
		  }]
		}
	}`))
	assert.Nil(t, err)
	newctx := newContext()
	err = newctx.AddImageInfos(resource1, cfg)
	assert.Nil(t, err)
	// images not loaded
	assert.Nil(t, newctx.images)
	// images loaded on Query
	name, err := newctx.Query("images.containers.test_container.name")
	assert.Nil(t, err)
	assert.Equal(t, name, "nginx")
}

func Test_ImageInfoLoader_OnDirectCall(t *testing.T) {
	resource1, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "name": "test-pod",
		  "namespace": "default"
		},
		"spec": {
		  "containers": [{
			"name": "test_container",
			"image": "nginx:latest"
		  }]
		}
	}`))
	assert.Nil(t, err)
	newctx := newContext()
	err = newctx.AddImageInfos(resource1, cfg)
	assert.Nil(t, err)
	// images not loaded
	assert.Nil(t, newctx.images)
	// images loaded on explicit call to ImageInfo
	imageinfos := newctx.ImageInfo()
	assert.Equal(t, imageinfos["containers"]["test_container"].Name, "nginx")
}

func Test_ContextSizeLimit(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int64
		entries []struct {
			name string
			data string
		}
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:    "within limit",
			maxSize: 1024,
			entries: []struct {
				name string
				data string
			}{
				{name: "small", data: `"hello"`},
			},
			wantErr: false,
		},
		{
			name:    "exceeds limit single entry",
			maxSize: 10,
			entries: []struct {
				name string
				data string
			}{
				{name: "large", data: `"this is a string that exceeds the limit"`},
			},
			wantErr:        true,
			expectedErrMsg: "context size limit exceeded",
		},
		{
			name:    "exceeds limit cumulative",
			maxSize: 50,
			entries: []struct {
				name string
				data string
			}{
				{name: "first", data: `"first entry data"`},
				{name: "second", data: `"second entry data"`},
				{name: "third", data: `"third entry that pushes over"`},
			},
			wantErr:        true,
			expectedErrMsg: "context size limit exceeded",
		},
		{
			name:    "zero limit disables check",
			maxSize: 0,
			entries: []struct {
				name string
				data string
			}{
				{name: "large", data: `"this can be any size when limit is zero"`},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &context{
				jp:             jp,
				jsonRaw:        map[string]interface{}{},
				maxContextSize: tt.maxSize,
				deferred:       NewDeferredLoaders(),
			}

			var lastErr error
			for _, entry := range tt.entries {
				lastErr = ctx.AddContextEntry(entry.name, []byte(entry.data))
				if lastErr != nil {
					break
				}
			}

			if tt.wantErr {
				assert.Error(t, lastErr)
				assert.Contains(t, lastErr.Error(), tt.expectedErrMsg)
				// Verify it's the correct error type
				var sizeErr ContextSizeLimitExceededError
				assert.ErrorAs(t, lastErr, &sizeErr)
			} else {
				assert.NoError(t, lastErr)
			}
		})
	}
}

func Test_ContextSizeLimitWithReplace(t *testing.T) {
	ctx := &context{
		jp:             jp,
		jsonRaw:        map[string]interface{}{},
		maxContextSize: 30,
		deferred:       NewDeferredLoaders(),
	}

	// First entry should succeed
	err := ctx.ReplaceContextEntry("var1", []byte(`"a"`))
	assert.NoError(t, err)
	assert.Greater(t, ctx.contextSize, int64(0))

	// Second entry should succeed
	err = ctx.ReplaceContextEntry("var2", []byte(`"b"`))
	assert.NoError(t, err)

	// Large entry that exceeds limit should fail
	largeData := []byte(`"this string is definitely larger than 30 bytes total"`)
	err = ctx.ReplaceContextEntry("var3", largeData)
	assert.Error(t, err)
	var sizeErr ContextSizeLimitExceededError
	assert.ErrorAs(t, err, &sizeErr)
}

func Test_ContextSizeLimitExceededError(t *testing.T) {
	err := ContextSizeLimitExceededError{Size: 3000, Limit: 2000}
	assert.Equal(t, "context size limit exceeded: 3000 bytes exceeds limit of 2000 bytes", err.Error())
}

// Test_ContextSizeLimitBlocksExponentialAmplification simulates a case where
// exponential string doubling via context variables attempts to consume
// unbounded memory (e.g., 1KB -> 2KB -> 4KB -> ... -> 256MB).
// This test verifies that the context size limit blocks such attacks.
func Test_ContextSizeLimitBlocksExponentialAmplification(t *testing.T) {
	// Use a small limit to make the test fast (16KB instead of 2MB default)
	const testLimit = 16 * 1024

	ctx := &context{
		jp:             jp,
		jsonRaw:        map[string]interface{}{},
		maxContextSize: testLimit,
		deferred:       NewDeferredLoaders(),
	}

	// Simulate the pattern:
	// l0 = random('[a-zA-Z0-9]{1000}') -> ~1KB
	// l1 = join('', [l0, l0]) -> ~2KB
	// l2 = join('', [l1, l1]) -> ~4KB
	// ... exponential growth until blocked

	baseString := strings.Repeat("a", 1000)
	currentData := baseString

	var lastErr error
	level := 0

	for level < 20 { // Would reach 1GB if unchecked
		jsonData, err := stdjson.Marshal(currentData)
		assert.NoError(t, err)

		entryName := fmt.Sprintf("l%d", level)
		lastErr = ctx.AddContextEntry(entryName, jsonData)

		if lastErr != nil {
			// Attack blocked by size limit
			break
		}

		// Double the string for next iteration (simulating join('', [prev, prev]))
		currentData = currentData + currentData
		level++
	}

	// Verify the attack was blocked before reaching dangerous levels
	assert.Error(t, lastErr, "exponential amplification should be blocked")
	assert.Less(t, level, 20, "attack should be blocked well before 20 doublings (1GB)")

	var sizeErr ContextSizeLimitExceededError
	assert.ErrorAs(t, lastErr, &sizeErr)
	assert.LessOrEqual(t, sizeErr.Limit, int64(testLimit))
}

// -----------------------------------------------------------------------
// #14526 regression suite.
//
// #14526 was a checkpoint-corruption bug: a nested map under a reserved
// key (for example request.object) was shared by reference between
// jsonRaw and a checkpoint, so mutating it after Checkpoint() also
// mutated the checkpoint. It was fixed by making Checkpoint() deep-copy
// the entire context (commit 5d5345ec3), at the cost of an O(context
// size) copy on every Checkpoint/Reset. #14526 itself had no regression
// test before this suite.
//
// These tests pin the invariant that fix established — a checkpoint is
// fully independent memory, so nothing written to the live context after
// Checkpoint() can ever become visible through Restore() — and they must
// keep passing under this file's current behavior (the full deep copy in
// copyContext).
//
// A scoped copy-on-write optimization was designed and implemented to
// avoid the full-copy cost, but was withdrawn after adversarial review
// found two independent ways it broke this exact invariant (see the
// "Escalation 2026-09-13" section of design.md for the analysis and the
// two reproductions, encoded below as
// Test_CheckpointNotCorruptedByScalarThenAliasInstall and
// Test_CheckpointNotCorruptedByNestedAliasInjection). This suite is the
// mandatory regression gate any future copy-on-write attempt must pass,
// under adversarial review, before it can replace the full deep copy.
// -----------------------------------------------------------------------

func mustAddResource(t *testing.T, ctx Interface, raw string) {
	t.Helper()
	var data map[string]interface{}
	if err := stdjson.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	if err := ctx.AddResource(data); err != nil {
		t.Fatal(err)
	}
}

// Test_CheckpointNotCorruptedByLaterMutation is the core #14526 regression
// test: a mutation after Checkpoint() must not be visible after Restore().
func Test_CheckpointNotCorruptedByLaterMutation(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)

	ctx.Checkpoint()

	// Mutate to object B via AddResource (rule-2 semantics): this
	// exercises both clearLeafValue and the mergeMaps reference-recursion
	// path that caused #14526.
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-b","labels":{"app":"b"}}}`)

	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-b", name)

	ctx.Restore()

	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)

	// Assert on a nested leaf, not just the top level: nested aliasing
	// was the actual #14526 bug.
	label, err := ctx.Query("request.object.metadata.labels.app")
	assert.NoError(t, err)
	assert.Equal(t, "a", label)
}

// Test_CheckpointNestedCheckpointsNotCorrupted guards that multiple
// checkpoint levels are each independent: restoring an inner checkpoint
// must reveal exactly the state at the time it was taken, not state from
// a different level.
func Test_CheckpointNestedCheckpointsNotCorrupted(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint()
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-b"}}`)

	ctx.Checkpoint()
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-c"}}`)

	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-c", name)

	ctx.Restore()
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-b", name)

	ctx.Restore()
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_CheckpointResetForeachPattern guards the engine's foreach pattern
// (one Checkpoint(), then repeated AddElement/Reset() per iteration):
// state written in one iteration must not leak into the next, and the
// original pre-loop state must still be there after the final Restore().
func Test_CheckpointResetForeachPattern(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint()

	assert.NoError(t, ctx.AddElement("e1", 0, 0))
	el, err := ctx.Query("element")
	assert.NoError(t, err)
	assert.Equal(t, "e1", el)

	ctx.Reset()
	_, err = ctx.Query("element")
	assert.Error(t, err, "element from the first iteration must not leak past Reset")

	assert.NoError(t, ctx.AddElement("e2", 1, 0))
	el, err = ctx.Query("element")
	assert.NoError(t, err)
	assert.Equal(t, "e2", el)

	ctx.Reset()
	_, err = ctx.Query("element")
	assert.Error(t, err, "element from the second iteration must not leak past Reset")

	ctx.Restore()
	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_CheckpointRestoreThenMutate guards a nested checkpoint/restore
// sequence followed by further mutation: after Restore() pops an inner
// checkpoint, a write must land only on the outer checkpoint's live
// state, never leaking into (or being mixed up with) the outer
// checkpoint itself, however many levels are involved.
func Test_CheckpointRestoreThenMutate(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint() // outer checkpoint (L0)
	ctx.Checkpoint() // inner checkpoint (L1)

	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-b"}}`)
	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-b", name)

	// Pops L1, restoring to the state at the time L1 was taken (pod-a).
	ctx.Restore()

	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-c"}}`)
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-c", name)

	// Pops L0; must show the original A, not the pod-c mutation above.
	ctx.Restore()
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_CheckpointTargetRestoresOriginal guards the "target" key, which is
// not matched by ReservedKeys, to confirm it is still protected
// identically to every other key now that copyContext deep-copies every
// key unconditionally (the ReservedKeys distinction is dead code; see
// the comment on copyContext).
func Test_CheckpointTargetRestoresOriginal(t *testing.T) {
	ctx := NewContext(jp)
	assert.NoError(t, ctx.SetTargetResource(map[string]interface{}{"metadata": map[string]interface{}{"name": "t1"}}))

	ctx.Checkpoint()

	assert.NoError(t, ctx.SetTargetResource(map[string]interface{}{"metadata": map[string]interface{}{"name": "t2"}}))
	name, err := ctx.Query("target.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "t2", name)

	ctx.Restore()
	name, err = ctx.Query("target.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "t1", name)
}

// Test_CheckpointAddVariableAndContextEntryRestoresOriginal guards
// non-reserved key names written via AddVariable/AddContextEntry.
func Test_CheckpointAddVariableAndContextEntryRestoresOriginal(t *testing.T) {
	ctx := NewContext(jp)
	assert.NoError(t, ctx.AddVariable("myVar.nested", "original"))
	assert.NoError(t, ctx.AddContextEntry("myEntry", []byte(`{"field":"original"}`)))

	ctx.Checkpoint()

	assert.NoError(t, ctx.AddVariable("myVar.nested", "mutated"))
	assert.NoError(t, ctx.AddContextEntry("myEntry", []byte(`{"field":"mutated"}`)))

	val, err := ctx.Query("myVar.nested")
	assert.NoError(t, err)
	assert.Equal(t, "mutated", val)

	entryVal, err := ctx.Query("myEntry.field")
	assert.NoError(t, err)
	assert.Equal(t, "mutated", entryVal)

	ctx.Restore()

	val, err = ctx.Query("myVar.nested")
	assert.NoError(t, err)
	assert.Equal(t, "original", val)

	entryVal, err = ctx.Query("myEntry.field")
	assert.NoError(t, err)
	assert.Equal(t, "original", entryVal)
}

// Test_CheckpointNotCorruptedByReinstalledQueryResult guards the
// invariant: a checkpoint must survive a live Query() result being
// reinstalled through any writer and then mutated through the context.
// This exact sequence broke a withdrawn scoped copy-on-write prototype
// (top-level-key privatization bookkeeping could be left stale after a
// by-reference install), because the prototype's checkpoints could still
// share subtrees with the live context between writes. It must pass
// under full deep copy (this file's current behavior), because every
// checkpoint is independent memory from the moment it's taken.
//
// Sequence:
//  1. AddResource(A) — request.object.metadata exists.
//  2. Checkpoint().
//  3. Query("request.object.metadata") returns a live reference into the
//     live context.
//  4. AddElement installs that live reference under "element" BY
//     REFERENCE (AddElement uses overwriteMaps=true, so mergeMaps
//     assigns destMap[k] = v directly instead of merging).
//  5. AddVariable("element.injected", ...) writes into "element".
//  6. Restore() must show the original, uninjected metadata — nothing
//     from steps 4-5 may be visible on the checkpoint.
func Test_CheckpointNotCorruptedByReinstalledQueryResult(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)

	ctx.Checkpoint()

	meta, err := ctx.Query("request.object.metadata")
	assert.NoError(t, err)
	_, isMap := meta.(map[string]interface{})
	assert.True(t, isMap, "request.object.metadata must be a live map[string]interface{} for this repro")

	// Installs the live query result under "element" by reference
	// (AddElement uses overwriteMaps=true).
	assert.NoError(t, ctx.AddElement(meta, 0, 0))

	// Writes into the same top-level key ("element") that now aliases
	// request.object.metadata.
	assert.NoError(t, ctx.AddVariable("element.injected", "boom"))

	injected, err := ctx.Query("element.injected")
	assert.NoError(t, err)
	assert.Equal(t, "boom", injected)

	ctx.Restore()

	// The checkpoint's request.object.metadata must be untouched by the
	// mutation above: no "injected" key should have leaked into it.
	_, err = ctx.Query("request.object.metadata.injected")
	assert.Error(t, err, "checkpoint corrupted: request.object.metadata.injected leaked from the aliased element write")

	// The original metadata must still be intact (not replaced, not
	// partially wiped by the corrupting mutation).
	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)

	label, err := ctx.Query("request.object.metadata.labels.app")
	assert.NoError(t, err)
	assert.Equal(t, "a", label)
}

// Test_CheckpointNotCorruptedByScalarThenAliasInstall guards the same
// invariant as Test_CheckpointNotCorruptedByReinstalledQueryResult — a
// checkpoint must survive a live Query() result being reinstalled through
// any writer and then mutated through the context — via a second
// reproduction that broke the withdrawn scoped copy-on-write prototype
// through a different path: a key first written with a scalar value (no
// map to privatize), then, under the SAME checkpoint, reinstalled with a
// live Query() result by reference, then mutated. It must pass under
// full deep copy (this file's current behavior). See the "Escalation
// 2026-09-13" section of design.md for the original analysis
// ("scalar-then-alias stale mark").
func Test_CheckpointNotCorruptedByScalarThenAliasInstall(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)

	ctx.Checkpoint()

	// First write under "element" is a scalar: no map to copy.
	assert.NoError(t, ctx.AddElement("scalar", 0, 0))

	meta, err := ctx.Query("request.object.metadata")
	assert.NoError(t, err)
	_, isMap := meta.(map[string]interface{})
	assert.True(t, isMap, "request.object.metadata must be a live map[string]interface{} for this repro")

	// Reinstalls "element" with a live query result, by reference
	// (AddElement uses overwriteMaps=true).
	assert.NoError(t, ctx.AddElement(meta, 1, 0))

	// Writes into "element" again, now aliasing request.object.metadata.
	assert.NoError(t, ctx.AddVariable("element.injected", "boom"))

	ctx.Restore()

	_, err = ctx.Query("request.object.metadata.injected")
	assert.Error(t, err, "checkpoint corrupted: request.object.metadata.injected leaked via the scalar-then-alias install path")

	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_CheckpointNotCorruptedByNestedAliasInjection guards the same
// invariant as the two tests above — a checkpoint must survive a live
// Query() result being reinstalled through any writer and then mutated
// through the context — via a third reproduction that broke the
// withdrawn scoped copy-on-write prototype even after its destination
// key had been correctly privatized: mergeMaps recursion can plant a
// live, aliased NESTED map at arbitrary depth inside an
// already-privatized destination, not just at the top level. It must
// pass under full deep copy (this file's current behavior). See the
// "Escalation 2026-09-13" section of design.md for the original analysis
// ("nested-alias injection").
func Test_CheckpointNotCorruptedByNestedAliasInjection(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)
	assert.NoError(t, ctx.AddVariable("x.placeholder", "p"))

	ctx.Checkpoint()

	meta, err := ctx.Query("request.object.metadata")
	assert.NoError(t, err)
	_, isMap := meta.(map[string]interface{})
	assert.True(t, isMap, "request.object.metadata must be a live map[string]interface{} for this repro")

	// Merges the live query result into the existing "x" map (created
	// before the checkpoint): mergeMaps recurses into "x" and plants
	// meta's own nested "labels" map by reference, since "x" has no
	// "labels" key yet.
	assert.NoError(t, ctx.AddVariable("x", meta))

	// Writes through the newly-planted alias.
	assert.NoError(t, ctx.AddVariable("x.labels.injected", "boom"))

	ctx.Restore()

	_, err = ctx.Query("request.object.metadata.labels.injected")
	assert.Error(t, err, `checkpoint corrupted: request.object.metadata.labels.injected leaked via nested alias injection under "x"`)

	label, err := ctx.Query("request.object.metadata.labels.app")
	assert.NoError(t, err)
	assert.Equal(t, "a", label)
}

// -----------------------------------------------------------------------
// Confirming micro-benchmark for the #17507 hypothesis: Checkpoint()/
// Reset() deep-copying the entire admission context is a measurable
// allocation cost on the legacy ClusterPolicy path. No Pod-sized JSON
// fixture exists in the repo, so a realistic Pod is constructed in-code.
// -----------------------------------------------------------------------

// benchmarkPod returns a fresh, realistic Pod-sized map: 2 containers with
// env vars and volume mounts, volumes, several labels/annotations, and a
// populated status block. Each call returns an independent map so
// benchmark iterations do not alias mutable state.
func benchmarkPod(name string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
			"uid":       "11111111-2222-3333-4444-555555555555",
			"labels": map[string]interface{}{
				"app":                          "sample-app",
				"app.kubernetes.io/name":       "sample-app",
				"app.kubernetes.io/instance":   "sample-app-abc123",
				"app.kubernetes.io/version":    "1.2.3",
				"app.kubernetes.io/component":  "backend",
				"app.kubernetes.io/part-of":    "sample-suite",
				"app.kubernetes.io/managed-by": "helm",
				"tier":                         "backend",
			},
			"annotations": map[string]interface{}{
				"kubernetes.io/psp":                  "restricted",
				"cni.projectcalico.org/podIP":        "10.244.1.23/32",
				"cni.projectcalico.org/podIPs":       "10.244.1.23/32",
				"kubectl.kubernetes.io/last-applied": `{"apiVersion":"v1","kind":"Pod"}`,
				"prometheus.io/scrape":               "true",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "docker.io/library/sample-app:1.2.3",
					"env": []interface{}{
						map[string]interface{}{"name": "ENV_ONE", "value": "value-one"},
						map[string]interface{}{"name": "ENV_TWO", "value": "value-two"},
						map[string]interface{}{"name": "ENV_THREE", "value": "value-three"},
					},
					"ports": []interface{}{
						map[string]interface{}{"containerPort": int64(8080), "name": "http"},
					},
					"volumeMounts": []interface{}{
						map[string]interface{}{"name": "config", "mountPath": "/etc/config"},
						map[string]interface{}{"name": "data", "mountPath": "/var/lib/data"},
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
						"limits":   map[string]interface{}{"cpu": "500m", "memory": "512Mi"},
					},
				},
				map[string]interface{}{
					"name":  "sidecar",
					"image": "docker.io/library/sample-sidecar:1.0.0",
					"env": []interface{}{
						map[string]interface{}{"name": "SIDECAR_ENV_ONE", "value": "sidecar-value-one"},
					},
					"volumeMounts": []interface{}{
						map[string]interface{}{"name": "config", "mountPath": "/etc/sidecar-config"},
					},
				},
			},
			"volumes": []interface{}{
				map[string]interface{}{
					"name":      "config",
					"configMap": map[string]interface{}{"name": "sample-app-config"},
				},
				map[string]interface{}{
					"name":     "data",
					"emptyDir": map[string]interface{}{},
				},
			},
			"serviceAccountName": "sample-app-sa",
			"nodeName":           "node-1",
		},
		"status": map[string]interface{}{
			"phase": "Running",
			"conditions": []interface{}{
				map[string]interface{}{"type": "Initialized", "status": "True"},
				map[string]interface{}{"type": "Ready", "status": "True"},
				map[string]interface{}{"type": "ContainersReady", "status": "True"},
				map[string]interface{}{"type": "PodScheduled", "status": "True"},
			},
			"containerStatuses": []interface{}{
				map[string]interface{}{
					"name":         "app",
					"ready":        true,
					"restartCount": int64(0),
					"image":        "docker.io/library/sample-app:1.2.3",
				},
				map[string]interface{}{
					"name":         "sidecar",
					"ready":        true,
					"restartCount": int64(0),
					"image":        "docker.io/library/sample-sidecar:1.0.0",
				},
			},
		},
	}
}

func benchmarkAdmissionRequest(pod map[string]interface{}) admissionv1.AdmissionRequest {
	raw, err := stdjson.Marshal(pod)
	if err != nil {
		panic(err)
	}
	return admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Object:    runtime.RawExtension{Raw: raw},
		OldObject: runtime.RawExtension{Raw: raw},
	}
}

// BenchmarkCheckpointRestore models a matched rule's handler invocation:
// Checkpoint(), mutate request.object (AddResource, as engine.go does
// after applying a patch), then Restore(). This quantifies the full
// deep-copy cost copyContext pays for a Pod-sized context when a write
// follows Checkpoint(). Under lazy checkpoint materialization
// (materializeCheckpoints), that cost is merely deferred to the first
// write rather than avoided — the copy count is identical to eager, so
// this benchmark is expected to stay flat versus base within noise. It
// is also the confirming measurement for the #17507 hypothesis (see
// design.md).
func BenchmarkCheckpointRestore(b *testing.B) {
	pod := benchmarkPod("bench-pod")
	mutatedPod := benchmarkPod("bench-pod")
	mutatedPod["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["mutated"] = "true"

	ctx := NewContext(jp)
	if err := ctx.AddRequest(benchmarkAdmissionRequest(pod)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Checkpoint()
		if err := ctx.AddResource(mutatedPod); err != nil {
			b.Fatal(err)
		}
		ctx.Restore()
	}
}

// BenchmarkCheckpointRestoreNoWrite models a matched rule whose handler
// does not mutate the context (the common case for policies without a
// mutation/generation side effect): Checkpoint() then Restore() only.
// Under lazy checkpoint materialization this is the case that gets
// cheap: Checkpoint() only pushes a nil (pending) marker, and Restore()
// of a pending entry is a content no-op that just pops the stack — no
// copyContext call happens at all, so allocations should drop from the
// eager ~159 allocs / ~21.3 KB down to ~0-1 allocs / sub-100 ns.
func BenchmarkCheckpointRestoreNoWrite(b *testing.B) {
	pod := benchmarkPod("bench-pod")

	ctx := NewContext(jp)
	if err := ctx.AddRequest(benchmarkAdmissionRequest(pod)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Checkpoint()
		ctx.Restore()
	}
}

// BenchmarkCheckpointResetForeach models the foreach pattern exercised by
// Test_CheckpointResetForeachPattern: one Checkpoint() followed by N
// Reset()-bounded iterations that each add a small per-iteration element
// (AddElement uses overwriteMaps=true, so this never touches "request").
// The first AddElement materializes the checkpoint (one copyContext
// call), and each subsequent Reset() re-deep-copies the now-materialized
// checkpoint (see copyContext) — identical copy count to eager, so under
// lazy checkpoint materialization this benchmark is expected to stay
// flat versus base within noise. Reset re-arm (making a materialized
// entry pending again to remove these copies) is deliberately out of
// scope for v1; see design.md's rejected alternatives.
func BenchmarkCheckpointResetForeach(b *testing.B) {
	pod := benchmarkPod("bench-pod")
	const iterations = 10

	ctx := NewContext(jp)
	if err := ctx.AddRequest(benchmarkAdmissionRequest(pod)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Checkpoint()
		for j := 0; j < iterations; j++ {
			if err := ctx.AddElement(j, j, 0); err != nil {
				b.Fatal(err)
			}
			ctx.Reset()
		}
		ctx.Restore()
	}
}

// -----------------------------------------------------------------------
// Lazy checkpoint materialization tests.
//
// Checkpoint() now records a "pending" (nil) entry in jsonRawCheckpoints
// instead of eagerly deep-copying; the entire contiguous nil suffix is
// materialized into independent copyContext deep copies on the first
// subsequent write (see materializeCheckpoints in context.go). These
// white-box tests (package context) pin the invariants that make that
// safe: I1 (pendings are a contiguous top suffix), I2 (a pending entry's
// state equals live), and the independence of materialized entries from
// each other and from live once materialized.
// -----------------------------------------------------------------------

// Test_LazyCheckpointPushesNilEntry guards the core mechanism change:
// Checkpoint() must not perform a deep copy up front — it only pushes a
// nil placeholder.
func Test_LazyCheckpointPushesNilEntry(t *testing.T) {
	ctx := NewContext(jp).(*context)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint()

	assert.Len(t, ctx.jsonRawCheckpoints, 1)
	assert.Nil(t, ctx.jsonRawCheckpoints[0], "Checkpoint() must record a pending (nil) entry, not a deep copy")
}

// Test_LazyNestedWriteMaterializesAllPending is the walkthrough's write
// case (design.md section 4): two nested checkpoints followed by a
// single write must materialize BOTH pending entries as independent deep
// copies of the pre-write state, not just the innermost one.
func Test_LazyNestedWriteMaterializesAllPending(t *testing.T) {
	ctx := NewContext(jp).(*context)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)

	ctx.Checkpoint() // outer (P)
	ctx.Checkpoint() // inner (R)

	// Both entries are still pending before any write.
	assert.Len(t, ctx.jsonRawCheckpoints, 2)
	assert.Nil(t, ctx.jsonRawCheckpoints[0])
	assert.Nil(t, ctx.jsonRawCheckpoints[1])

	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-b","labels":{"app":"b"}}}`)

	// A single write must materialize the entire pending suffix.
	assert.NotNil(t, ctx.jsonRawCheckpoints[0], "outer checkpoint must be materialized by the write")
	assert.NotNil(t, ctx.jsonRawCheckpoints[1], "inner checkpoint must be materialized by the write")

	// Independence: mutating live deeply after materialization must not
	// affect either materialized entry.
	liveMeta := ctx.jsonRaw["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})
	liveMeta["labels"].(map[string]interface{})["mutated"] = "yes"

	outerName := ctx.jsonRawCheckpoints[0]["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["name"]
	assert.Equal(t, "pod-a", outerName)
	_, outerHasMutated := ctx.jsonRawCheckpoints[0]["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["mutated"]
	assert.False(t, outerHasMutated, "outer checkpoint must be independent of a later live mutation")

	innerName := ctx.jsonRawCheckpoints[1]["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["name"]
	assert.Equal(t, "pod-a", innerName)
	_, innerHasMutated := ctx.jsonRawCheckpoints[1]["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["mutated"]
	assert.False(t, innerHasMutated, "inner checkpoint must be independent of a later live mutation")

	// The two materialized entries must also be independent of each
	// other: mutating one must not affect the other.
	ctx.jsonRawCheckpoints[1]["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["name"] = "corrupted"
	assert.Equal(t, "pod-a", ctx.jsonRawCheckpoints[0]["request"].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["name"])

	// Restore(inner), write, Restore(outer) must return the two correct
	// prior states (steps 5-6 of the walkthrough).
	ctx.Restore()
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-c"}}`)
	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-c", name)

	ctx.Restore()
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_LazyRestoreInnerPendingThenOuterMaterialized covers the reachable
// mixed-nesting case: outer materialized by an earlier write, inner
// pending because no write happened under it. Restore(inner) must be a
// content no-op; Restore(outer) must return the pre-write state.
func Test_LazyRestoreInnerPendingThenOuterMaterialized(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint()                                                                        // outer
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-b"}}`) // materializes outer with pod-a

	ctx.Checkpoint() // inner: pending, live == pod-b

	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-b", name)

	// No write under the inner checkpoint.
	ctx.Restore() // pops inner (pending): no-op on content
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-b", name)

	ctx.Restore() // pops outer (materialized): returns pod-a
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_LazyNoWriteCheckpointRestoreIsNoop guards that Checkpoint()/
// Restore() with no intervening write costs nothing observable: content
// is unchanged and the checkpoint stack is empty afterwards, both for a
// single checkpoint and nested two-deep.
func Test_LazyNoWriteCheckpointRestoreIsNoop(t *testing.T) {
	ctx := NewContext(jp).(*context)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint()
	ctx.Restore()

	assert.Len(t, ctx.jsonRawCheckpoints, 0)
	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)

	// Nested, two-deep, still no write.
	ctx.Checkpoint()
	ctx.Checkpoint()
	assert.Len(t, ctx.jsonRawCheckpoints, 2)
	ctx.Restore()
	assert.Len(t, ctx.jsonRawCheckpoints, 1)
	ctx.Restore()
	assert.Len(t, ctx.jsonRawCheckpoints, 0)

	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
}

// Test_LazyResetOnPendingCheckpoint guards the Reset()-on-pending rule
// (R6 in design.md): Reset() of a pending entry must leave it pending
// (not pop, not materialize). After a write materializes it, Reset()
// reverts to the base copyContext behavior.
func Test_LazyResetOnPendingCheckpoint(t *testing.T) {
	ctx := NewContext(jp).(*context)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a"}}`)

	ctx.Checkpoint()
	assert.Nil(t, ctx.jsonRawCheckpoints[0])

	ctx.Reset() // no-op: stays pending, depth unchanged
	assert.Len(t, ctx.jsonRawCheckpoints, 1)
	assert.Nil(t, ctx.jsonRawCheckpoints[0])
	name, err := ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)

	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-b"}}`)
	assert.NotNil(t, ctx.jsonRawCheckpoints[0], "write must materialize the checkpoint")

	ctx.Reset() // materialized branch: base behavior, restores checkpoint content
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
	assert.Len(t, ctx.jsonRawCheckpoints, 1, "Reset must not pop the checkpoint")

	ctx.Restore()
	name, err = ctx.Query("request.object.metadata.name")
	assert.NoError(t, err)
	assert.Equal(t, "pod-a", name)
	assert.Len(t, ctx.jsonRawCheckpoints, 0)
}

// Test_LazyDeferredLoaderLevels guards that nil placeholders keep
// len(jsonRawCheckpoints)-keyed deferred loader levels identical to
// eager, in both the "loader fires" and "loader never fires" variants,
// under nested pending checkpoints.
func Test_LazyDeferredLoaderLevels(t *testing.T) {
	// Variant 1: loader added under a checkpoint, fires (queried) before
	// Restore, then Restore must reload it at the outer level.
	t.Run("fired", func(t *testing.T) {
		ctx := newContext()
		AddMockDeferredLoader(ctx, "outerVal", "0")

		ctx.Checkpoint() // level 1, pending
		mock, _ := AddMockDeferredLoader(ctx, "value", "1")

		val, err := ctx.Query("value")
		assert.NoError(t, err)
		assert.Equal(t, "1", val)
		assert.Equal(t, 1, mock.invocations)

		// Query triggers addJSON -> materializes the pending checkpoint
		// before installing "value" in jsonRaw.
		assert.NotNil(t, ctx.jsonRawCheckpoints[0])

		ctx.Restore()
		// After restore, "value" should be gone (loader was added at the
		// restored level and removed).
		_, err = ctx.Query("value")
		assert.Error(t, err)
	})

	// Variant 2: loader added under a nested pending checkpoint, never
	// queried; Restore must simply remove it with no materialization
	// artifacts leaking.
	t.Run("unfired", func(t *testing.T) {
		ctx := newContext()

		ctx.Checkpoint() // outer, pending
		ctx.Checkpoint() // inner, pending
		unused, _ := AddMockDeferredLoader(ctx, "unused", "unused")

		assert.Nil(t, ctx.jsonRawCheckpoints[0])
		assert.Nil(t, ctx.jsonRawCheckpoints[1])

		ctx.Restore() // pops inner: pending no-op
		assert.Equal(t, 0, unused.invocations)

		ctx.Restore() // pops outer: pending no-op
		assert.Equal(t, 0, unused.invocations)

		_, err := ctx.Query("unused")
		assert.Error(t, err, "loader added under a popped checkpoint must not survive")
	})
}

// Test_LazyEngineDeferPattern models engine.go's per-rule defer exactly
// (design.md section 4, the "does not write" walkthrough): a policy-level
// checkpoint (P) and a rule-level checkpoint (R), Restore(R), then
// AddResource(same pod) as engine.go's defer unconditionally does, then
// Restore(P). The final state must equal the original — this is
// precisely the step that corrupted checkpoints before #14600, and it
// guards the "honest arithmetic" correctness (policy-level checkpoint
// materializes once via the defer's AddResource) from design.md's
// performance reality check.
func Test_LazyEngineDeferPattern(t *testing.T) {
	ctx := NewContext(jp)
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)

	original, err := ctx.Query("request.object")
	assert.NoError(t, err)
	originalCopy := runtime.DeepCopyJSON(original.(map[string]interface{}))

	ctx.Checkpoint() // P: policy-level
	ctx.Checkpoint() // R: rule-level

	// Handler runs, no writes (PSS-style validate-only rule).
	ctx.Restore() // pops R (pending): no-op

	// engine.go's defer unconditionally re-adds the (possibly patched)
	// resource; here it is the same object, but this still triggers
	// materialization of P.
	mustAddResource(t, ctx, `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-a","labels":{"app":"a"}}}`)

	ctx.Restore() // pops P (materialized): must return the true original

	final, err := ctx.Query("request.object")
	assert.NoError(t, err)
	assert.Equal(t, originalCopy, final)
}
