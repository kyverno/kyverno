package context

import (
	"reflect"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
)

func Test_addResourceAndUserContext(t *testing.T) {
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
		Username: "system:serviceaccount:nirmata:toledo-damien-gmail-com-binding",
		UID:      "014fbff9a07c",
	}
	userRequestInfo := kyverno.RequestInfo{
		Roles:             nil,
		ClusterRoles:      nil,
		AdmissionUserInfo: userInfo}

	var expectedResult string
	ctx := NewContext()
	ctx.AddResource(rawResource)
	result, err := ctx.Query("request.object.apiVersion")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "v1"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}

	ctx.AddUserInfo(userRequestInfo)
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
	expectedResult = "system:serviceaccount:nirmata:toledo-damien-gmail-com-binding"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}
	// Add service account
	ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	result, err = ctx.Query("serviceAccount")
	if err != nil {
		t.Error(err)
	}
	expectedResult = "nirmata:toledo-damien-gmail-com-binding"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}
}
