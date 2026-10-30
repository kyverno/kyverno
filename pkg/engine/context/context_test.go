package context

import (
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/toggle"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// Test_ReplaceContextEntryDoesNotLeakSize is a regression test for
// https://github.com/kyverno/kyverno/issues/17408.
// Replacing the same context entry repeatedly (as happens during foreach iterations)
// must not accumulate contextSize. Before the fix, contextSize only ever grew on each
// call to ReplaceContextEntry, eventually triggering a spurious
// ContextSizeLimitExceededError even when the actual live data was small.
func Test_ReplaceContextEntryDoesNotLeakSize(t *testing.T) {
	const limit = 100
	ctx := &context{
		jp:             jp,
		jsonRaw:        map[string]interface{}{},
		maxContextSize: limit,
		deferred:       NewDeferredLoaders(),
	}

	// Simulate a foreach loop that replaces the same context variable many times.
	// Each payload is ~20 bytes; 10 iterations would sum to ~200 bytes (> limit)
	// if contextSize were never decremented — but since we replace the same key each
	// time, the net size should stay at ~20 bytes and never exceed the 100-byte limit.
	payload := []byte(`"iteration-value-xx"`)
	for i := range 10 {
		err := ctx.ReplaceContextEntry("loopVar", payload)
		assert.NoError(t, err, "iteration %d should not exceed the context size limit", i)
	}

	// The running contextSize must not exceed the limit.
	assert.LessOrEqual(t, ctx.contextSize, int64(limit),
		"contextSize should not grow unboundedly when the same entry is replaced repeatedly")
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

func TestContext_CheckpointRestoreImages(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"imagePullSecrets": []interface{}{
					map[string]interface{}{
						"name": "my-secret",
					},
				},
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	err := ctx.AddImageInfos(pod, cfg)
	assert.NoError(t, err)

	imagesBefore := ctx.ImageInfo()
	assert.Equal(t, 1, len(imagesBefore["containers"]))
	assert.Equal(t, []string{"my-secret"}, imagesBefore["containers"]["app"].ImagePullSecrets)

	ctx.Checkpoint()

	// Mutate ImagePullSecrets slice in ctx.images to verify deep copy isolation
	info := ctx.images["containers"]["app"]
	info.ImagePullSecrets[0] = "mutated-secret"
	info.ImagePullSecrets = append(info.ImagePullSecrets, "extra-secret")
	ctx.images["containers"]["app"] = info

	customPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"data": map[string]interface{}{
				"myImage": "redis:6.0",
			},
		},
	}
	customExtractors := kyvernov1.ImageExtractorConfigs{
		"Pod": []kyvernov1.ImageExtractorConfig{
			{
				Path: "/data/myImage",
				Name: "customImg",
			},
		},
	}
	_, err = ctx.GenerateCustomImageInfo(customPod, customExtractors, cfg)
	assert.NoError(t, err)

	imagesInsideCheckpoint := ctx.ImageInfo()
	assert.Equal(t, 1, len(imagesInsideCheckpoint["customImg"]))

	// Test Reset()
	ctx.Reset()
	imagesAfterReset := ctx.ImageInfo()
	assert.Equal(t, 1, len(imagesAfterReset["containers"]))
	assert.Equal(t, []string{"my-secret"}, imagesAfterReset["containers"]["app"].ImagePullSecrets)
	assert.Nil(t, imagesAfterReset["customImg"])

	// Verify deferred loading on Query("images...") does not overwrite the reset image map
	queryVal, err := ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "nginx", queryVal)
	assert.Nil(t, ctx.ImageInfo()["customImg"])

	// Mutate again before Restore
	info2 := ctx.images["containers"]["app"]
	info2.ImagePullSecrets[0] = "mutated-again"
	ctx.images["containers"]["app"] = info2

	ctx.Restore()

	imagesAfterRestore := ctx.ImageInfo()
	assert.Equal(t, 1, len(imagesAfterRestore["containers"]))
	appInfo := imagesAfterRestore["containers"]["app"]
	assert.Equal(t, "docker.io/nginx:1.20", appInfo.String())
	assert.Equal(t, []string{"my-secret"}, appInfo.ImagePullSecrets)
	assert.Nil(t, imagesAfterRestore["customImg"])

	// Verify deferred loading on Query("images...") after Restore does not re-extract or overwrite
	queryVal2, err := ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "nginx", queryVal2)
	assert.Nil(t, ctx.ImageInfo()["customImg"])
}

func TestContext_DeferredImageLoader_DoesNotOverwriteRestoredEmptyContext(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	err := ctx.AddImageInfos(pod, cfg)
	assert.NoError(t, err)

	// Images are not loaded initially; loader has not loaded yet.
	assert.Nil(t, ctx.images)

	// Create a checkpoint where ctx.images is empty
	ctx.Checkpoint()

	// Inside checkpoint, custom images are generated (populating ctx.images)
	customPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"data": map[string]interface{}{
				"myImage": "redis:6.0",
			},
		},
	}
	customExtractors := kyvernov1.ImageExtractorConfigs{
		"Pod": []kyvernov1.ImageExtractorConfig{
			{
				Path: "/data/myImage",
				Name: "customImg",
			},
		},
	}
	_, err = ctx.GenerateCustomImageInfo(customPod, customExtractors, cfg)
	assert.NoError(t, err)

	// A query for images occurs inside the checkpoint
	queryVal, err := ctx.Query("images.customImg")
	assert.NoError(t, err)
	assert.NotNil(t, queryVal)
	assert.True(t, ctx.imageInfoLoader.HasLoaded())

	// Restore the checkpoint - should restore ctx.images to empty without reloading original pod
	ctx.Restore()

	// After restore, the loader's loaded state is restored to false
	assert.False(t, ctx.imageInfoLoader.HasLoaded())
	// Before querying images, ctx.images should remain nil (not populated with stale pod images)
	assert.Nil(t, ctx.images)

	// Explicit call to ImageInfo() after restore loads the original pod
	images := ctx.ImageInfo()
	assert.True(t, ctx.imageInfoLoader.HasLoaded())
	assert.Equal(t, 1, len(images["containers"]))
	assert.Equal(t, "nginx", images["containers"]["app"].Name)
	assert.Nil(t, images["customImg"])
}

func TestContext_DeferredImageLoader_RetainsLoadedStateWhenPopulatedBeforeCheckpoint(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	err := ctx.AddImageInfos(pod, cfg)
	assert.NoError(t, err)

	customPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"data": map[string]interface{}{
				"myImage": "redis:6.0",
			},
		},
	}
	customExtractors := kyvernov1.ImageExtractorConfigs{
		"Pod": []kyvernov1.ImageExtractorConfig{
			{
				Path: "/data/myImage",
				Name: "customImg",
			},
		},
	}
	// Custom images populated before checkpoint
	_, err = ctx.GenerateCustomImageInfo(customPod, customExtractors, cfg)
	assert.NoError(t, err)

	// Query sets hasLoaded to true
	_, err = ctx.Query("images.customImg")
	assert.NoError(t, err)
	assert.True(t, ctx.imageInfoLoader.HasLoaded())

	ctx.Checkpoint()
	assert.True(t, ctx.imageInfoLoader.HasLoaded())

	ctx.Restore()
	// After restore, loader retains loaded state since it was loaded prior to checkpoint
	assert.True(t, ctx.imageInfoLoader.HasLoaded())
	assert.NotNil(t, ctx.ImageInfo()["customImg"])
}

func TestContext_GenerateCustomImageInfo_SynchronizesRawContextAndClearsRemovedImages(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "c1",
						"image": "nginx:1.20",
					},
					map[string]interface{}{
						"name":  "c2",
						"image": "busybox:latest",
					},
				},
			},
		},
	}

	// Populate initial images
	images, err := ctx.GenerateCustomImageInfo(pod, nil, cfg)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(images["containers"]))

	// Query both containers in context
	c1Img, err := ctx.Query("images.containers.c1.name")
	assert.NoError(t, err)
	assert.Equal(t, "nginx", c1Img)

	c2Img, err := ctx.Query("images.containers.c2.name")
	assert.NoError(t, err)
	assert.Equal(t, "busybox", c2Img)

	// Simulate mutation that removed container c1 and only keeps c2
	mutatedPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "c2",
						"image": "busybox:latest",
					},
				},
			},
		},
	}

	imagesMutated, err := ctx.GenerateCustomImageInfo(mutatedPod, nil, cfg)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(imagesMutated["containers"]))

	// Removed container c1 must no longer exist in jsonRaw context
	_, err = ctx.Query("images.containers.c1")
	assert.ErrorContains(t, err, `Unknown key "c1" in path`)

	// Kept container c2 remains accessible
	c2After, err := ctx.Query("images.containers.c2.name")
	assert.NoError(t, err)
	assert.Equal(t, "busybox", c2After)

	// When all images are removed (e.g. empty spec), images context must be cleared
	emptyPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{},
		},
	}
	imagesEmpty, err := ctx.GenerateCustomImageInfo(emptyPod, nil, cfg)
	assert.NoError(t, err)
	assert.Empty(t, imagesEmpty)
	assert.Nil(t, ctx.images)

	_, err = ctx.Query("images")
	assert.ErrorContains(t, err, `Unknown key "images" in path`)
}

func TestContext_DeferredImageLoader_DoesNotResurrectImagesAfterEmptyRefreshWithoutPriorQuery(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	// Add deferred loader with initial pod containing nginx.
	// Intentionally DO NOT query images here, so hasLoaded remains false.
	err := ctx.AddImageInfos(pod, cfg)
	assert.NoError(t, err)
	assert.False(t, ctx.imageInfoLoader.HasLoaded())

	// Mutation removes all containers, so the resource now has no images.
	emptyPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{},
		},
	}

	// GenerateCustomImageInfo is called for the mutated resource before any image query.
	images, err := ctx.GenerateCustomImageInfo(emptyPod, nil, cfg)
	assert.NoError(t, err)
	assert.Empty(t, images)
	assert.True(t, ctx.imageInfoLoader.HasLoaded())

	// A later query for images must NOT resurrect images from the original pod via deferred loader.
	_, err = ctx.Query("images.containers.app")
	assert.ErrorContains(t, err, `Unknown key "images" in path`)

	// Explicit call to ImageInfo() must also not reload the original pod.
	assert.Empty(t, ctx.ImageInfo())
}

func TestContext_DeferredImageLoader_ClearsMatchesOnAddResource(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "pod1",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	cfg := config.NewDefaultConfiguration(false)
	err := ctx.AddImageInfos(pod1, cfg)
	assert.NoError(t, err)

	// Query images to trigger deferred loader, setting dl.matched = true
	queryVal, err := ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "nginx", queryVal)
	assert.True(t, ctx.imageInfoLoader.HasLoaded())

	// AddResource with a new pod containing a different image
	pod2 := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "pod2",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "redis:6.0",
				},
			},
		},
	}
	err = ctx.AddResource(pod2)
	assert.NoError(t, err)
	assert.False(t, ctx.imageInfoLoader.HasLoaded())

	// Querying images again should re-trigger deferred loader because matches were cleared
	queryVal, err = ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "redis", queryVal)
	assert.True(t, ctx.imageInfoLoader.HasLoaded())
}

func TestContext_AddResource_ClearsImageStateWithoutImageInfoLoader(t *testing.T) {
	ctx := NewContext(jp).(*context)
	pod1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "pod1",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	cfg := config.NewDefaultConfiguration(false)
	_, err := ctx.GenerateCustomImageInfo(pod1, nil, cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, ctx.ImageInfo())

	queryVal, err := ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "nginx", queryVal)

	// AddResource with a new pod without imageInfoLoader installed
	pod2 := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "pod2",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "redis:6.0",
				},
			},
		},
	}
	err = ctx.AddResource(pod2)
	assert.NoError(t, err)

	// Images state must be cleared
	assert.Empty(t, ctx.ImageInfo())
	_, err = ctx.Query("images.containers.app.name")
	assert.Error(t, err)
}

func TestContext_ImageInfoLoader_ReloadsOnAddResourceWhenDeferredLoadingDisabled(t *testing.T) {
	_ = toggle.EnableDeferredLoading.Parse("false")
	t.Cleanup(func() {
		_ = toggle.EnableDeferredLoading.Parse("true")
	})

	ctx := NewContext(jp).(*context)
	pod1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "pod1",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app",
						"image": "nginx:1.20",
					},
				},
			},
		},
	}
	cfg := config.NewDefaultConfiguration(false)
	err := ctx.AddImageInfos(pod1, cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, ctx.ImageInfo())

	queryVal, err := ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "nginx", queryVal)

	// AddResource with a new pod while deferred loading is disabled
	pod2 := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": "pod2",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "redis:6.0",
				},
			},
		},
	}
	err = ctx.AddResource(pod2)
	assert.NoError(t, err)

	// Images must be eagerly reloaded for pod2
	assert.NotEmpty(t, ctx.ImageInfo())
	queryVal, err = ctx.Query("images.containers.app.name")
	assert.NoError(t, err)
	assert.Equal(t, "redis", queryVal)
}
