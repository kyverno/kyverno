package patch

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/mattbaird/jsonpatch"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
)

func Test_GeneratePatches(t *testing.T) {

	out, err := strategicMergePatch(logging.GlobalLogger(), string(baseBytes), string(overlayBytes))
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
			path:   "/metadata/ownerReferences",
			ignore: false,
		},
		{
			path:   "/metadata/finalizers",
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
		{
			path:   "/spec/triggers/0/metadata/serverAddress",
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
		{
			patches: []jsonpatch.JsonPatchOperation{
				{Operation: "remove", Path: "/spec/containers/0/args/7"},
				{Operation: "remove", Path: "/spec/containers/0/args/6"},
				{Operation: "remove", Path: "/spec/containers/0/args/5"},
				{Operation: "remove", Path: "/spec/containers/0/args/4"},
				{Operation: "remove", Path: "/spec/containers/0/args/3"},
				{Operation: "remove", Path: "/spec/containers/0/args/2"},
				{Operation: "remove", Path: "/spec/containers/0/args/1"},
				{Operation: "remove", Path: "/spec/containers/0/args/0"},
				{Operation: "add", Path: "/spec/containers/0/args/0", Value: "--logtostderr"},
				{Operation: "add", Path: "/spec/containers/0/args/1", Value: "--secure-listen-address=[$(IP)]:9100"},
				{Operation: "add", Path: "/spec/containers/0/args/2", Value: "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256"},
				{Operation: "add", Path: "/spec/containers/0/args/3", Value: "--upstream=http://127.0.0.1:9100/"},
				{Operation: "remove", Path: "/spec/containers/1/args/3"},
				{Operation: "remove", Path: "/spec/containers/1/args/2"},
				{Operation: "remove", Path: "/spec/containers/1/args/1"},
				{Operation: "remove", Path: "/spec/containers/1/args/0"},
				{Operation: "add", Path: "/spec/containers/1/args/0", Value: "--web.listen-address=127.0.0.1:9100"},
				{Operation: "add", Path: "/spec/containers/1/args/1", Value: "--Path.procfs=/host/proc"},
				{Operation: "add", Path: "/spec/containers/1/args/2", Value: "--Path.sysfs=/host/sys"},
				{Operation: "add", Path: "/spec/containers/1/args/3", Value: "--Path.rootfs=/host/root"},
				{Operation: "add", Path: "/spec/containers/1/args/4", Value: "--no-collector.wifi"},
				{Operation: "add", Path: "/spec/containers/1/args/5", Value: "--no-collector.hwmon"},
				{Operation: "add", Path: "/spec/containers/1/args/6", Value: "--collector.filesystem.ignored-mount-points=^/(dev|proc|sys|var/lib/docker/.+)($|/)"},
				{Operation: "add", Path: "/spec/containers/1/args/7", Value: "--collector.filesystem.ignored-fs-types=^(autofs|binfmt_misc|cgroup|tracefs)$"},
			},
			expected: []jsonpatch.JsonPatchOperation{
				{Operation: "remove", Path: "/spec/containers/0/args/0"},
				{Operation: "remove", Path: "/spec/containers/0/args/1"},
				{Operation: "remove", Path: "/spec/containers/0/args/2"},
				{Operation: "remove", Path: "/spec/containers/0/args/3"},
				{Operation: "remove", Path: "/spec/containers/0/args/4"},
				{Operation: "remove", Path: "/spec/containers/0/args/5"},
				{Operation: "remove", Path: "/spec/containers/0/args/6"},
				{Operation: "remove", Path: "/spec/containers/0/args/7"},
				{Operation: "add", Path: "/spec/containers/0/args/0", Value: "--logtostderr"},
				{Operation: "add", Path: "/spec/containers/0/args/1", Value: "--secure-listen-address=[$(IP)]:9100"},
				{Operation: "add", Path: "/spec/containers/0/args/2", Value: "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256"},
				{Operation: "add", Path: "/spec/containers/0/args/3", Value: "--upstream=http://127.0.0.1:9100/"},
				{Operation: "remove", Path: "/spec/containers/1/args/0"},
				{Operation: "remove", Path: "/spec/containers/1/args/1"},
				{Operation: "remove", Path: "/spec/containers/1/args/2"},
				{Operation: "remove", Path: "/spec/containers/1/args/3"},
				{Operation: "add", Path: "/spec/containers/1/args/0", Value: "--web.listen-address=127.0.0.1:9100"},
				{Operation: "add", Path: "/spec/containers/1/args/1", Value: "--Path.procfs=/host/proc"},
				{Operation: "add", Path: "/spec/containers/1/args/2", Value: "--Path.sysfs=/host/sys"},
				{Operation: "add", Path: "/spec/containers/1/args/3", Value: "--Path.rootfs=/host/root"},
				{Operation: "add", Path: "/spec/containers/1/args/4", Value: "--no-collector.wifi"},
				{Operation: "add", Path: "/spec/containers/1/args/5", Value: "--no-collector.hwmon"},
				{Operation: "add", Path: "/spec/containers/1/args/6", Value: "--collector.filesystem.ignored-mount-points=^/(dev|proc|sys|var/lib/docker/.+)($|/)"},
				{Operation: "add", Path: "/spec/containers/1/args/7", Value: "--collector.filesystem.ignored-fs-types=^(autofs|binfmt_misc|cgroup|tracefs)$"},
			},
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
