package mutate

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestMergePatch(t *testing.T) {
	testCases := []struct {
		rawPolicy   []byte
		rawResource []byte
		expected    []byte
	}{
		{
			rawPolicy:   overlayBytes,
			rawResource: baseBytes,
			expected:    expectBytes,
		},
		{
			// condition matches the first element of the array
			rawPolicy: []byte(`{
        "spec": {
          "containers": [
            {
              "(image)": "gcr.io/google-containers/busybox:*"
            }
          ],
          "imagePullSecrets": [
            {
              "name": "regcred"
            }
          ]
        }
      }`),
			rawResource: []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "hello"
        },
        "spec": {
          "containers": [
            {
              "name": "hello",
              "image": "gcr.io/google-containers/busybox:latest"
            },
            {
              "name": "hello2",
              "image": "gcr.io/google-containers/busybox:latest"
            },
            {
              "name": "hello3",
              "image": "gcr.io/google-containers/nginx:latest"
            }
          ]
        }
      }`),
			expected: []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "hello"
        },
        "spec": {
          "containers": [
            {
              "image": "gcr.io/google-containers/busybox:latest",
              "name": "hello"
            },
            {
              "image": "gcr.io/google-containers/busybox:latest",
              "name": "hello2"
            },
            {
              "image": "gcr.io/google-containers/nginx:latest",
              "name": "hello3"
            }
          ],
          "imagePullSecrets": [
            {
              "name": "regcred"
            }
          ]
        }
      }`),
		},
		{
			// condition matches the third element of the array
			rawPolicy: []byte(`{
        "spec": {
          "containers": [
            {
              "(image)": "gcr.io/google-containers/nginx:*"
            }
          ],
          "imagePullSecrets": [
            {
              "name": "regcred"
            }
          ]
        }
      }`),
			rawResource: []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "hello"
        },
        "spec": {
          "containers": [
            {
              "name": "hello",
              "image": "gcr.io/google-containers/busybox:latest"
            },
            {
              "name": "hello2",
              "image": "gcr.io/google-containers/busybox:latest"
            },
            {
              "name": "hello3",
              "image": "gcr.io/google-containers/nginx:latest"
            }
          ]
        }
      }`),
			expected: []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "hello"
        },
        "spec": {
          "containers": [
            {
              "image": "gcr.io/google-containers/busybox:latest",
              "name": "hello"
            },
            {
              "image": "gcr.io/google-containers/busybox:latest",
              "name": "hello2"
            },
            {
              "image": "gcr.io/google-containers/nginx:latest",
              "name": "hello3"
            }
          ],
          "imagePullSecrets": [
            {
              "name": "regcred"
            }
          ]
        }
      }`),
		},
	}

	for i, test := range testCases {
		t.Logf("Running test %d...", i+1)
		out, err := strategicMergePatch(log.Log, string(test.rawResource), string(test.rawPolicy))
		assert.NilError(t, err)

		// has assertions inside
		areEqualJSONs(t, test.expected, out)
	}
}

func Test_PolicyDeserilize(t *testing.T) {
	rawPolicy := []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "set-image-pull-policy"
  },
  "spec": {
    "validationFailureAction": "enforce",
    "rules": [
      {
        "name": "set-image-pull-policy",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "mutate": {
          "patchStrategicMerge": {
            "spec": {
              "template": {
                "spec": {
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
                  ],
                  "initContainers": [
                    {
                      "name": "init-command",
                      "image": "debian",
                      "command": [
                        "echo $(WORDPRESS_SERVICE)",
                        "echo $(MYSQL_SERVICE)"
                      ]
                    }
                  ]
                }
              }
            }
          }
        }
      }
    ]
  }
}
`)

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	overlayPatches := policy.Spec.Rules[0].Mutation.PatchStrategicMerge
	patchString, err := json.Marshal(overlayPatches)
	assert.NilError(t, err)

	out, err := strategicMergePatch(log.Log, string(baseBytes), string(patchString))
	assert.NilError(t, err)

	var ep unstructured.Unstructured
	err = json.Unmarshal(expectBytes, &ep)
	assert.NilError(t, err)

	eb, err := json.Marshal(ep.Object)
	assert.NilError(t, err)

	if !assertnew.Equal(t, string(eb), string(out)) {
		t.FailNow()
	}
}
