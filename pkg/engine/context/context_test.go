package context

import (
	"reflect"
	"testing"

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
		Username: "admin",
		UID:      "014fbff9a07c",
	}

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

	ctx.AddUserInfo(userInfo)
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
	expectedResult = "admin"
	t.Log(result)
	if !reflect.DeepEqual(expectedResult, result) {
		t.Error("exected result does not match")
	}
}
