package patch

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMergePatch(t *testing.T) {
	var expectBytes = []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "wordpress","labels": {"app": "wordpress"}},"spec": {"selector": {"matchLabels": {"app": "wordpress"}},"strategy": {"type": "Recreate"},"template": {"metadata": {"labels": {"app": "wordpress"}},"spec": {"containers": [{"name": "nginx","image": "nginx"},{"image": "wordpress:4.8-apache","name": "wordpress","ports": [{"containerPort": 80,"name": "wordpress"}],"volumeMounts": [{"name": "wordpress-persistent-storage","mountPath": "/var/www/html"}],"env": [{"name": "WORDPRESS_DB_HOST","value": "$(MYSQL_SERVICE)"},{"name": "WORDPRESS_DB_PASSWORD","valueFrom": {"secretKeyRef": {"name": "mysql-pass","key": "password"}}}]}],"volumes": [{"name": "wordpress-persistent-storage"}],"initContainers": [{"name": "init-command","image": "debian","command": ["echo $(WORDPRESS_SERVICE)","echo $(MYSQL_SERVICE)"]}]}}}}`)
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
		out, err := strategicMergePatch(logr.Discard(), string(test.rawResource), string(test.rawPolicy))
		assert.NilError(t, err)
		assert.DeepEqual(t, toJSON(t, test.expected), toJSON(t, out))
	}
}

func Test_PolicyDeserilize(t *testing.T) {
	var expectBytes = []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "wordpress","labels": {"app": "wordpress"}},"spec": {"selector": {"matchLabels": {"app": "wordpress"}},"strategy": {"type": "Recreate"},"template": {"metadata": {"labels": {"app": "wordpress"}},"spec": {"containers": [{"name": "nginx","image": "nginx"},{"image": "wordpress:4.8-apache","name": "wordpress","ports": [{"containerPort": 80,"name": "wordpress"}],"volumeMounts": [{"name": "wordpress-persistent-storage","mountPath": "/var/www/html"}],"env": [{"name": "WORDPRESS_DB_HOST","value": "$(MYSQL_SERVICE)"},{"name": "WORDPRESS_DB_PASSWORD","valueFrom": {"secretKeyRef": {"name": "mysql-pass","key": "password"}}}]}],"volumes": [{"name": "wordpress-persistent-storage"}],"initContainers": [{"name": "init-command","image": "debian","command": ["echo $(WORDPRESS_SERVICE)","echo $(MYSQL_SERVICE)"]}]}}}}`)
	rawPolicy := []byte(`
{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "set-image-pull-policy"
  },
  "spec": {
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

	overlayPatches := autogen.Default.ComputeRules(&policy, "")[0].Mutation.GetPatchStrategicMerge()
	patchString, err := json.Marshal(overlayPatches)
	assert.NilError(t, err)

	out, err := strategicMergePatch(logr.Discard(), string(baseBytes), string(patchString))
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

func TestMergePatchControlCharacters(t *testing.T) {
	// encoding/json leaves these unescaped
	for _, c := range []string{"\u007f", "\u0080", "\u0085", "\u009f", "\ufffe", "a\u007fb", "\u00a0"} {
		t.Run(fmt.Sprintf("%+q", c), func(t *testing.T) {
			value, err := json.Marshal(c)
			assert.NilError(t, err)
			base := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"containers":[{"name":"c","env":[{"name":"X","value":` + string(value) + `}]}]}}`
			overlay := `{"metadata":{"labels":{"foo":"bar","baz":` + string(value) + `}}}`
			expected := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","labels":{"foo":"bar","baz":` + string(value) + `}},"spec":{"containers":[{"name":"c","env":[{"name":"X","value":` + string(value) + `}]}]}}`
			out, err := strategicMergePatch(logr.Discard(), base, overlay)
			assert.NilError(t, err)
			assert.DeepEqual(t, toJSON(t, []byte(expected)), toJSON(t, out))
		})
	}
}

func TestMergePatchMalformed(t *testing.T) {
	valid := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"}}`
	malformed := `{"metadata": [}`
	_, err := strategicMergePatch(logr.Discard(), valid, malformed)
	assert.ErrorContains(t, err, "failed to preProcess rule")
	_, err = strategicMergePatch(logr.Discard(), malformed, valid)
	assert.ErrorContains(t, err, "failed to preProcess rule")
}
