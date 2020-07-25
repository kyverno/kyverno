package mutate

import (
	"testing"

	"github.com/mattbaird/jsonpatch"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
)

func Test_GeneratePatches(t *testing.T) {
	expectedPatches := []jsonpatch.JsonPatchOperation{
		{
			Operation: "remove",
			Path:      "/spec/template/spec/containers/0",
			Value:     nil,
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/containers/0",
			Value: map[string]interface{}{
				"image": "nginx",
				"name":  "nginx",
			},
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/containers/1",
			Value: map[string]interface{}{
				"env": []interface{}{
					map[string]interface{}{
						"name":  "WORDPRESS_DB_HOST",
						"value": "$(MYSQL_SERVICE)",
					},
					map[string]interface{}{
						"name": "WORDPRESS_DB_PASSWORD",
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"key":  "password",
								"name": "mysql-pass",
							},
						},
					},
				},
				"image": "wordpress:4.8-apache",
				"name":  "wordpress",
				"ports": []interface{}{
					map[string]interface{}{
						"containerPort": float64(80),
						"name":          "wordpress",
					},
				},
				"volumeMounts": []interface{}{
					map[string]interface{}{
						"mountPath": "/var/www/html",
						"name":      "wordpress-persistent-storage",
					},
				},
			},
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/initContainers",
			Value: []interface{}{
				map[string]interface{}{
					"command": []interface{}{
						"echo $(WORDPRESS_SERVICE)",
						"echo $(MYSQL_SERVICE)",
					},
					"image": "debian",
					"name":  "init-command",
				},
			},
		},
	}

	out, err := strategicMergePatchfilter()
	assert.NilError(t, err)

	patches, err := generatePatches("", out)
	assert.NilError(t, err)

	if !assertnew.Equal(t, expectedPatches, patches) {
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
                        "name": "wordpress-persistent-storage",
                        "emptyDir": {}
                    }
                ]
            }
        }
    }
}
`)
var expectBytes = []byte(`
{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "labels": {
            "app": "wordpress"
        },
        "name": "wordpress"
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
                        "image": "nginx",
                        "name": "nginx"
                    },
                    {
                        "env": [
                            {
                                "name": "WORDPRESS_DB_HOST",
                                "value": "$(MYSQL_SERVICE)"
                            },
                            {
                                "name": "WORDPRESS_DB_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "password",
                                        "name": "mysql-pass"
                                    }
                                }
                            }
                        ],
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
                                "mountPath": "/var/www/html",
                                "name": "wordpress-persistent-storage"
                            }
                        ]
                    }
                ],
                "initContainers": [
                    {
                        "command": [
                            "echo $(WORDPRESS_SERVICE)",
                            "echo $(MYSQL_SERVICE)"
                        ],
                        "image": "debian",
                        "name": "init-command"
                    }
                ],
                "volumes": [
                    {
                        "emptyDir": {},
                        "name": "wordpress-persistent-storage"
                    }
                ]
            }
        }
    }
}
`)
