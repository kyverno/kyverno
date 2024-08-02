package context

import (
	"reflect"
	"testing"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
)

var jp = jmespath.New(config.NewDefaultConfiguration(false))

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
