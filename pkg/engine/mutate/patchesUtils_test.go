package mutate

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mattbaird/jsonpatch"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
	yaml "sigs.k8s.io/yaml"
)

func Test_GeneratePatches(t *testing.T) {

	out, err := strategicMergePatch(string(baseBytes), string(overlayBytes))
	assert.NilError(t, err)

	expectedPatches := map[string]bool{
		`{"op":"remove","path":"/spec/template/spec/containers/0"}`:                                       true,
		`{"op":"add","path":"/spec/template/spec/containers/0","value":{"image":"nginx","name":"nginx"}}`: true,
		`{"op":"add","path":"/spec/template/spec/containers/1","value":{"env":[{"name":"WORDPRESS_DB_HOST","value":"$(MYSQL_SERVICE)"},{"name":"WORDPRESS_DB_PASSWORD","valueFrom":{"secretKeyRef":{"key":"password","name":"mysql-pass"}}}],"image":"wordpress:4.8-apache","name":"wordpress","ports":[{"containerPort":80,"name":"wordpress"}],"volumeMounts":[{"mountPath":"/var/www/html","name":"wordpress-persistent-storage"}]}}`: true,
		`{"op":"add","path":"/spec/template/spec/initContainers","value":[{"command":["echo $(WORDPRESS_SERVICE)","echo $(MYSQL_SERVICE)"],"image":"debian","name":"init-command"}]}`:                                                                                                                                                                                                                                                    true,
	}
	patches, err := generatePatches(baseBytes, out)
	assert.NilError(t, err)

	for _, p := range patches {
		assertnew.Equal(t, expectedPatches[string(p)], true)
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
	patchesJSON6902 := []byte(`
- op: add
  path: /spec/containers/1
  value: {"name":"nginx-new","image":"nginx:latest"}
`)
	var pod unstructured.Unstructured
	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))

	patches, err := yaml.YAMLToJSON(patchesJSON6902)
	assertnew.Nil(t, err)

	skip, err := preProcessJSONPatches(patches, pod, log.Log)
	assertnew.Nil(t, err)
	assertnew.Equal(t, true, skip)
}

func Test_preProcessJSONPatches_not_skip(t *testing.T) {
	patchesJSON6902 := []byte(`
- op: add
  path: /spec/containers/1
  value: {"name":"my-new-container","image":"nginx:latest"}
`)

	patches, err := yaml.YAMLToJSON(patchesJSON6902)
	assertnew.Nil(t, err)

	var pod unstructured.Unstructured
	assertnew.Nil(t, json.Unmarshal(podBytes, &pod))

	skip, err := preProcessJSONPatches(patches, pod, log.Log)
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

func Test_ignorePath(t *testing.T) {
	tests := []struct {
		path   string
		ignore bool
	}{
		{
			path:   "/",
			ignore: false,
		},
		{
			path:   "/metadata",
			ignore: false,
		},
		{
			path:   "/metadata/name",
			ignore: false,
		},
		{
			path:   "spec/template/metadata/name",
			ignore: false,
		},
		{
			path:   "/metadata/namespace",
			ignore: false,
		},
		{
			path:   "/metadata/annotations",
			ignore: false,
		},
		{
			path:   "/metadata/labels",
			ignore: false,
		},
		{
			path:   "/metadata/creationTimestamp",
			ignore: true,
		},
		{
			path:   "spec/template/metadata/creationTimestamp",
			ignore: true,
		},
		{
			path:   "/metadata/resourceVersion",
			ignore: true,
		},
		{
			path:   "/status",
			ignore: true,
		},
		{
			path:   "/spec",
			ignore: false,
		},
		{
			path:   "/kind",
			ignore: false,
		},
	}

	for _, test := range tests {
		res := ignorePatch(test.path)
		assertnew.Equal(t, test.ignore, res, fmt.Sprintf("test fails at %s", test.path))
	}
}

func Test_GeneratePatches_sortRemovalPatches(t *testing.T) {
	base := []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "nginx-deployment","labels": {"app": "nginx"}},"spec": {"selector": {"matchLabels": {"app": "nginx"}},"replicas": 1,"template": {"metadata": {"labels": {"app": "nginx"}},"spec": {"containers": [{"name": "nginx","image": "nginx:1.14.2","ports": [{"containerPort": 80}]}],"tolerations": [{"effect": "NoExecute","key": "node.kubernetes.io/not-ready","operator": "Exists","tolerationSeconds": 300},{"effect": "NoExecute","key": "node.kubernetes.io/unreachable","operator": "Exists","tolerationSeconds": 300}]}}}}`)
	patchedResource := []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "nginx-deployment","labels": {"app": "nginx"}},"spec": {"selector": {"matchLabels": {"app": "nginx"}},"replicas": 1,"template": {"metadata": {"labels": {"app": "nginx"}},"spec": {"containers": [{"name": "nginx","image": "nginx:1.14.2","ports": [{"containerPort": 80}]}],"tolerations": [{"effect": "NoSchedule","key": "networkzone","operator": "Equal","value": "dmz"}]}}}}`)
	expectedPatches := [][]byte{[]byte(`{"op":"remove","path":"/spec/template/spec/tolerations/1"}`), []byte(`{"op":"remove","path":"/spec/template/spec/tolerations/0"}`), []byte(`{"op":"add","path":"/spec/template/spec/tolerations/0","value":{"effect":"NoSchedule","key":"networkzone","operator":"Equal","value":"dmz"}}`)}
	patches, err := generatePatches(base, patchedResource)
	fmt.Println(patches)
	assertnew.Nil(t, err)
	assertnew.Equal(t, expectedPatches, patches)

}

func Test_sortRemovalPatches(t *testing.T) {
	tests := []struct {
		patches  []jsonpatch.JsonPatchOperation
		expected []jsonpatch.JsonPatchOperation
	}{
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "add", Path: "/a"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "add", Path: "/a"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "add", Path: "/a"}, {Operation: "remove", Path: "/a"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "add", Path: "/a"}, {Operation: "remove", Path: "/a"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "add", Path: "/a/0"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "add", Path: "/a/0"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/a/1"}, {Operation: "remove", Path: "/a/2"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/2"}, {Operation: "remove", Path: "/a/1"}, {Operation: "remove", Path: "/a/0"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/b/0"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/b/0"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/b/0"}, {Operation: "remove", Path: "/b/1"}, {Operation: "remove", Path: "/c/0"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/b/1"}, {Operation: "remove", Path: "/b/0"}, {Operation: "remove", Path: "/c/0"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "add", Path: "/z"}, {Operation: "remove", Path: "/b/0"}, {Operation: "remove", Path: "/b/1"}, {Operation: "remove", Path: "/c/0"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "add", Path: "/z"}, {Operation: "remove", Path: "/b/1"}, {Operation: "remove", Path: "/b/0"}, {Operation: "remove", Path: "/c/0"}},
		},
		{
			patches:  []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/b/0"}, {Operation: "add", Path: "/b/c/0"}, {Operation: "remove", Path: "/b/1"}, {Operation: "remove", Path: "/c/0"}},
			expected: []jsonpatch.JsonPatchOperation{{Operation: "remove", Path: "/a/0"}, {Operation: "remove", Path: "/b/0"}, {Operation: "add", Path: "/b/c/0"}, {Operation: "remove", Path: "/b/1"}, {Operation: "remove", Path: "/c/0"}},
		},
	}

	for i, test := range tests {
		sortedPatches := filterAndSortPatches(test.patches)
		assertnew.Equal(t, test.expected, sortedPatches, fmt.Sprintf("%dth test fails", i))
	}
}

func Test_getRemoveInterval(t *testing.T) {
	tests := []struct {
		removalPaths  []string
		expectedIndex [][]int
	}{
		{
			removalPaths:  []string{"/a/0", "/b/0", "/b/1", "/c/0"},
			expectedIndex: [][]int{{1, 2}},
		},
		{
			removalPaths:  []string{},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a/0"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a/0", "/a/1"},
			expectedIndex: [][]int{{0, 1}},
		},
		{
			removalPaths:  []string{"/a/0", "/a"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a", "/a/0"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a", "/a"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a/0", "/b/0"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a/0", "/a/1", "/a/2"},
			expectedIndex: [][]int{{0, 2}},
		},
		{
			removalPaths:  []string{"/a/0", "/a/1", "/a/b"},
			expectedIndex: [][]int{{0, 1}},
		},
		{
			removalPaths:  []string{"/a", "/a/0", "/a/1"},
			expectedIndex: [][]int{{1, 2}},
		},
		{
			removalPaths:  []string{"/", "/a", "/b/0"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a/b", "/a/c", "/b/0", "/b/1", "/c"},
			expectedIndex: [][]int{{2, 3}},
		},
		{
			removalPaths:  []string{"/a/0", "/b/c", "/b/d", "/b/e", "/c/0"},
			expectedIndex: [][]int{},
		},
		{
			removalPaths:  []string{"/a/0", "/a/1", "/b/z", "/c/0", "/c/1", "/c/2", "/d/z", "/e/0"},
			expectedIndex: [][]int{{0, 1}, {3, 5}},
		},
		{
			removalPaths:  []string{"/a/0", "/a/1", "/a/2", "/a/3"},
			expectedIndex: [][]int{{0, 3}},
		},
	}

	for i, test := range tests {
		res := getRemoveInterval(test.removalPaths)
		assertnew.Equal(t, test.expectedIndex, res, fmt.Sprintf("%d-th test fails at path %v", i, test.removalPaths))
	}
}
