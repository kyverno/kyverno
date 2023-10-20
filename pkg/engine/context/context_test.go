package context

import (
	"reflect"
	"testing"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
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

func Test_addVariable(t *testing.T) {
	var err error

	rawResource := `
	apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: secret-annotation
spec:
  validationFailureAction: Enforce
  rules:
    - name: remove-annotation
      preconditions:
        all:
          - key: "{{ request.operation }}"
            operator: Equals
            value: UPDATE
          - key: "{{ request.oldObject.metadata.labels.\"com.example/my-label\" || '' }}"
            operator: Equals
            value: all
      match:
        all:
          - resources:
              kinds:
                - Secret
              annotations:
                com.example/my-annotation: ".*"
              selector:
                matchExpressions:
                  - key: com.example/my-label
                    operator: DoesNotExist
      mutate:
        patchesJson6902: |-
          - op: remove
            path: "/metadata/annotations/com.example~1my-annotation"
          `
	valuesField, err := parseValuesField(rawResource)
	if err != nil {
		t.Fatalf("Error parsing policy YAML: %v", err)
	}
	expectedValues := map[string]interface{}{

		"key": "{{ request.oldObject.metadata.labels.\"com.example/my-label\" || '' }}",
	}

	if !reflect.DeepEqual(expectedValues, valuesField) {
		t.Errorf("Expected values do not match actual values. Expected: %v, Actual: %v", expectedValues, valuesField)
	}

}
func parseValuesField(yaml string) (map[string]interface{}, error) {

	return map[string]interface{}{}, nil
}
