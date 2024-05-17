package patch

import (
	"testing"

	"github.com/go-logr/logr"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/stretchr/testify/assert"
)

func Test_GeneratePatches(t *testing.T) {
	out, err := strategicMergePatch(logr.Discard(), string(baseBytes), string(overlayBytes))
	assert.NoError(t, err)
	expectedPatches := map[string]bool{
		`{"op":"add","path":"/spec/template/spec/initContainers","value":[{"command":["echo $(WORDPRESS_SERVICE)","echo $(MYSQL_SERVICE)"],"image":"debian","name":"init-command"}]}`:                                                                                                                                                                                                                                                    true,
		`{"op":"add","path":"/spec/template/spec/containers/1","value":{"env":[{"name":"WORDPRESS_DB_HOST","value":"$(MYSQL_SERVICE)"},{"name":"WORDPRESS_DB_PASSWORD","valueFrom":{"secretKeyRef":{"key":"password","name":"mysql-pass"}}}],"image":"wordpress:4.8-apache","name":"wordpress","ports":[{"containerPort":80,"name":"wordpress"}],"volumeMounts":[{"mountPath":"/var/www/html","name":"wordpress-persistent-storage"}]}}`: true,
		`{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"nginx"}`: true,
		`{"op":"replace","path":"/spec/template/spec/containers/0/name","value":"nginx"}`:  true,
		`{"op":"remove","path":"/spec/template/spec/containers/0/ports"}`:                  true,
		`{"op":"remove","path":"/spec/template/spec/containers/0/volumeMounts"}`:           true,
	}
	patches, err := generatePatches(baseBytes, out)
	assert.NoError(t, err)
	for _, p := range patches {
		assert.Equal(t, expectedPatches[p.Json()], true)
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

func Test_GeneratePatches_sortRemovalPatches(t *testing.T) {
	base := []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "nginx-deployment","labels": {"app": "nginx"}},"spec": {"selector": {"matchLabels": {"app": "nginx"}},"replicas": 1,"template": {"metadata": {"labels": {"app": "nginx"}},"spec": {"containers": [{"name": "nginx","image": "nginx:1.14.2","ports": [{"containerPort": 80}]}],"tolerations": [{"effect": "NoExecute","key": "node.kubernetes.io/not-ready","operator": "Exists","tolerationSeconds": 300},{"effect": "NoExecute","key": "node.kubernetes.io/unreachable","operator": "Exists","tolerationSeconds": 300}]}}}}`)
	expectedResource := []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "nginx-deployment","labels": {"app": "nginx"}},"spec": {"selector": {"matchLabels": {"app": "nginx"}},"replicas": 1,"template": {"metadata": {"labels": {"app": "nginx"}},"spec": {"containers": [{"name": "nginx","image": "nginx:1.14.2","ports": [{"containerPort": 80}]}],"tolerations": [{"effect": "NoSchedule","key": "networkzone","operator": "Equal","value": "dmz"}]}}}}`)
	generatedPatches, err := generatePatches(base, expectedResource)
	assert.NoError(t, err)
	patchedResource, err := engineutils.ApplyPatches(base, ConvertPatches(generatedPatches...))
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedResource), string(patchedResource))
}
