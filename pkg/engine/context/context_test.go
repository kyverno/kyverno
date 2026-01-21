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
	authenticationv1 "k8s.io/api/authentication/v1"
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
