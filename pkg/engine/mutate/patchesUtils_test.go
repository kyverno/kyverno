package mutate

import (
	"encoding/json"
	"testing"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
	yaml "sigs.k8s.io/yaml"
)

func Test_GeneratePatches(t *testing.T) {

	out, err := strategicMergePatch(string(baseBytes), string(overlayBytes))
	assert.NilError(t, err)

	patches, err := generatePatches(baseBytes, out)
	assert.NilError(t, err)

	var overlay unstructured.Unstructured
	err = json.Unmarshal(baseBytes, &overlay)
	assert.NilError(t, err)

	bb, err := json.Marshal(overlay.Object)
	assert.NilError(t, err)

	res, err := utils.ApplyPatches(bb, patches)
	assert.NilError(t, err)

	var ep unstructured.Unstructured
	err = json.Unmarshal(expectBytes, &ep)
	assert.NilError(t, err)

	eb, err := json.Marshal(ep.Object)
	assert.NilError(t, err)

	if !assertnew.Equal(t, string(eb), string(res)) {
		t.FailNow()
	}
}

var baseBytes = []byte(`
{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "wordpress",
    "labels": {
      "app": "wordpress"
    }
  },
  "spec": {
    "selector": {
      "matchLabels": {
        "app": "wordpress"
      }
    },
    "strategy": {
      "type": "Recreate"
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "wordpress"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "wordpress:4.8-apache",
            "name": "wordpress",
            "ports": [
              {
                "containerPort": 80,
                "name": "wordpress"
              }
            ],
            "volumeMounts": [
              {
                "name": "wordpress-persistent-storage",
                "mountPath": "/var/www/html"
              }
            ]
          }
        ],
        "volumes": [
          {
            "name": "wordpress-persistent-storage"
          }
        ]
      }
    }
  }
}
`)

var overlayBytes = []byte(`
{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "wordpress"
  },
  "spec": {
    "template": {
      "spec": {
        "initContainers": [
          {
            "name": "init-command",
            "image": "debian",
            "command": [
              "echo $(WORDPRESS_SERVICE)",
              "echo $(MYSQL_SERVICE)"
            ]
          }
        ],
        "containers": [
          {
            "name": "nginx",
            "image": "nginx"
          },
          {
            "name": "wordpress",
            "env": [
              {
                "name": "WORDPRESS_DB_HOST",
                "value": "$(MYSQL_SERVICE)"
              },
              {
                "name": "WORDPRESS_DB_PASSWORD",
                "valueFrom": {
                  "secretKeyRef": {
                    "name": "mysql-pass",
                    "key": "password"
                  }
                }
              }
            ]
          }
        ]
      }
    }
  }
}
`)

var expectBytes = []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "wordpress","labels": {"app": "wordpress"}},"spec": {"selector": {"matchLabels": {"app": "wordpress"}},"strategy": {"type": "Recreate"},"template": {"metadata": {"labels": {"app": "wordpress"}},"spec": {"containers": [{"name": "nginx","image": "nginx"},{"image": "wordpress:4.8-apache","name": "wordpress","ports": [{"containerPort": 80,"name": "wordpress"}],"volumeMounts": [{"name": "wordpress-persistent-storage","mountPath": "/var/www/html"}],"env": [{"name": "WORDPRESS_DB_HOST","value": "$(MYSQL_SERVICE)"},{"name": "WORDPRESS_DB_PASSWORD","valueFrom": {"secretKeyRef": {"name": "mysql-pass","key": "password"}}}]}],"volumes": [{"name": "wordpress-persistent-storage"}],"initContainers": [{"name": "init-command","image": "debian","command": ["echo $(WORDPRESS_SERVICE)","echo $(MYSQL_SERVICE)"]}]}}}}`)

var podBytes = []byte(`
{
  "kind": "Pod",
  "apiVersion": "v1",
  "metadata": {
      "name": "nginx"
  },
  "spec": {
      "containers": [
          {
              "name": "nginx",
              "image": "nginx:latest"
          },
          {
            "name": "nginx-new",
            "image": "nginx:latest"
          }
      ]
  }
}
`)

func Test_preProcessJSONPatches_skip(t *testing.T) {
	var policyBytes = []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
      "name": "insert-container"
  },
  "spec": {
      "rules": [
          {
              "name": "insert-container",
              "match": {
                  "resources": {
                      "kinds": [
                          "Pod"
                      ]
                  }
              },
              "mutate": {
                  "patchesJson6902": "- op: add\n  path: /spec/containers/1\n  value: {\"name\":\"nginx-new\",\"image\":\"nginx:latest\"}"
              }
          }
      ]
  }
}
`)

	var pod unstructured.Unstructured
	var policy v1.ClusterPolicy

	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))
	assertnew.Nil(t, yaml.Unmarshal(policyBytes, &policy))

	skip, err := preProcessJSONPatches(policy.Spec.Rules[0].Mutation, pod, log.Log)
	assertnew.Nil(t, err)
	assertnew.Equal(t, true, skip)
}

func Test_preProcessJSONPatches_not_skip(t *testing.T) {
	var policyBytes = []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
      "name": "insert-container"
  },
  "spec": {
      "rules": [
          {
              "name": "insert-container",
              "match": {
                  "resources": {
                      "kinds": [
                          "Pod"
                      ]
                  }
              },
              "mutate": {
                  "patchesJson6902": "- op: add\n  path: /spec/containers/1\n  value: {\"name\":\"my-new-container\",\"image\":\"nginx:latest\"}"
              }
          }
      ]
  }
}
`)

	var pod unstructured.Unstructured
	var policy v1.ClusterPolicy

	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))
	assertnew.Nil(t, yaml.Unmarshal(policyBytes, &policy))

	skip, err := preProcessJSONPatches(policy.Spec.Rules[0].Mutation, pod, log.Log)
	assertnew.Nil(t, err)
	assertnew.Equal(t, false, skip)
}

func Test_isSubsetObject_true(t *testing.T) {
	var object, resource interface{}

	objectRaw := []byte(`{"image": "nginx:latest","name": "nginx-new"}`)
	resourceRaw := []byte(`{"image": "nginx:latest","name": "random-name"}`)
	assertnew.Nil(t, json.Unmarshal(objectRaw, &object))
	assertnew.Nil(t, json.Unmarshal(resourceRaw, &resource))
	assertnew.Equal(t, false, isSubsetObject(object, resource))

	resourceRawNew := []byte(`{"image": "nginx:latest","name": "nginx-new"}`)
	assertnew.Nil(t, json.Unmarshal(resourceRawNew, &resource))
	assertnew.Equal(t, true, isSubsetObject(object, resource))
}

func Test_getObject_notPresent(t *testing.T) {
	path := "/spec/random/1"
	var pod unstructured.Unstructured

	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))
	_, err := getObject(path, pod.UnstructuredContent())
	expectedErr := "referenced value does not exist at spec/random"
	assertnew.Equal(t, err.Error(), expectedErr)
}

func Test_getObject_outOfIndex(t *testing.T) {
	path := "/spec/containers/2"
	var pod unstructured.Unstructured

	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))
	object, err := getObject(path, pod.UnstructuredContent())
	assertnew.Nil(t, err)
	assertnew.Nil(t, object)

}

func Test_getObject_success(t *testing.T) {
	path := "/spec/containers/1"
	var pod unstructured.Unstructured
	expectedObject := map[string]interface{}{"image": "nginx:latest", "name": "nginx-new"}

	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))
	object, err := getObject(path, pod.UnstructuredContent())
	assertnew.Nil(t, err)
	assertnew.Equal(t, expectedObject, object)
}

func Test_getObject_get_last_element(t *testing.T) {
	path := "/spec/containers/-"
	var pod unstructured.Unstructured
	expectedObject := map[string]interface{}{"image": "nginx:latest", "name": "nginx-new"}

	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))
	object, err := getObject(path, pod.UnstructuredContent())
	assertnew.Nil(t, err)
	assertnew.Equal(t, expectedObject, object)
}
