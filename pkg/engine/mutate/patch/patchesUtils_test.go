package patch

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
)

func Test_GeneratePatches(t *testing.T) {
	out, err := strategicMergePatch(logr.Discard(), string(baseBytes), string(overlayBytes))
	assert.NilError(t, err)
	expectedPatches := map[string]bool{
		`{"op":"add","path":"/spec/template/spec/initContainers","value":[{"command":["echo $(WORDPRESS_SERVICE)","echo $(MYSQL_SERVICE)"],"image":"debian","name":"init-command"}]}`:                                                                                                                                                                                                                                                    true,
		`{"op":"add","path":"/spec/template/spec/containers/1","value":{"env":[{"name":"WORDPRESS_DB_HOST","value":"$(MYSQL_SERVICE)"},{"name":"WORDPRESS_DB_PASSWORD","valueFrom":{"secretKeyRef":{"key":"password","name":"mysql-pass"}}}],"image":"wordpress:4.8-apache","name":"wordpress","ports":[{"containerPort":80,"name":"wordpress"}],"volumeMounts":[{"mountPath":"/var/www/html","name":"wordpress-persistent-storage"}]}}`: true,
		`{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"nginx"}`: true,
		`{"op":"replace","path":"/spec/template/spec/containers/0/name","value":"nginx"}`:  true,
		`{"op":"remove","path":"/spec/template/spec/containers/0/ports"}`:                  true,
		`{"op":"remove","path":"/spec/template/spec/containers/0/volumeMounts"}`:           true,
	}
	patches, err := generatePatches(baseBytes, out)
	assert.NilError(t, err)
	for _, p := range patches {
		assertnew.Equal(t, expectedPatches[p.Json()], true)
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
			path:   "/metadata/generateName",
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
			ignore: false,
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
	expectedPatches := [][]byte{
		[]byte(`{"op":"remove","path":"/spec/template/spec/tolerations/1"}`),
		[]byte(`{"op":"replace","path":"/spec/template/spec/tolerations/0/effect","value":"NoSchedule"}`),
		[]byte(`{"op":"replace","path":"/spec/template/spec/tolerations/0/key","value":"networkzone"}`),
		[]byte(`{"op":"replace","path":"/spec/template/spec/tolerations/0/operator","value":"Equal"}`),
		[]byte(`{"op":"add","path":"/spec/template/spec/tolerations/0/value","value":"dmz"}`),
		[]byte(`{"op":"remove","path":"/spec/template/spec/tolerations/0/tolerationSeconds"}`),
	}
	patches, err := generatePatches(base, patchedResource)
	assertnew.Nil(t, err)
	for _, patch := range patches {
		fmt.Println(patch.Json())
	}
	assertnew.Equal(t, expectedPatches, ConvertPatches(patches...))
}
