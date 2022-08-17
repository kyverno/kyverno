package webhooks

import (
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"gotest.tools/assert"
)

func Test_updateFeildsInSourceAndUpdatedResource(t *testing.T) {
	type TestCase struct {
		obj            map[string]interface{}
		newRes         map[string]interface{}
		expectedObj    map[string]interface{}
		expectedNewRes map[string]interface{}
	}

	testcases := []TestCase{
		{
			obj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----\nMIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQEL\nBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ\n-----END CERTIFICATE-----",
				},
				"kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"imageregistry": "https://hub.docker.com/",
						"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","data":{"ca":"-----BEGIN CERTIFICATE-----\nMIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQEL\nBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ\n-----END CERTIFICATE-----"},"kind":"ConfigMap","metadata":{"annotations":{"imageregistry":"https://hub.docker.com/"},"name":"corp-ca-cert","namespace":"default"}}`,
					},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{"generate.kyverno.io/clone-policy-name": "generate-policy"},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			newRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----\nMIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQEL\nBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ\n-----END CERTIFICATE-----",
				},
				"kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			expectedObj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----MIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQELBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ-----END CERTIFICATE-----",
				},
				"kind": "ConfigMap",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"imageregistry": "https://hub.docker.com/",
					},
					"labels": map[string]interface{}{},
				},
			},

			expectedNewRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"ca": "-----BEGIN CERTIFICATE-----MIID5zCCAs+gAwIBAgIUCl6BKlpe2QiS5IQby6QOW7vexMwwDQYJKoZIhvcNAQELBQAwgYIxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTENMAsGA1UEBwwEVG93bjEQ-----END CERTIFICATE-----",
				},
				"kind":     "ConfigMap",
				"metadata": map[string]interface{}{},
			},
		},
		{
			obj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","data":{"tls.crt":"MIIC2DCCAcCgAwIBAgIBATANBgkqh","tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ"},"type": "kubernetes.io/tls","kind":"Secret"}`,
					},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{"generate.kyverno.io/clone-policy-name": "generate-policy"},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			newRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations":       map[string]interface{}{},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels": map[string]interface{}{
						"policy.kyverno.io/gr-name":     "gr-qmjr9",
						"policy.kyverno.io/policy-name": "generate-policy",
					},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
			},

			expectedObj: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels":      map[string]interface{}{},
				},
			},

			expectedNewRes: map[string]interface{}{
				"apiVersion": "v1",
				"data": map[string]interface{}{
					"tls.crt": "MIIC2DCCAcCgAwIBAgIBATANBgkqh",
					"tls.key": "MIIEpgIBAAKCAQEA7yn3bRHQ5FHMQ",
				},
				"kind": "Secret",
				"type": "kubernetes.io/tls",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels": map[string]interface{}{
						"policy.kyverno.io/gr-name":     "gr-qmjr9",
						"policy.kyverno.io/policy-name": "generate-policy",
					},
				},
			},
		},
		{
			obj: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Pod", ...}`,
					},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{"generate.kyverno.io/clone-policy-name": "generate-policy"},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
				"status": map[string]interface{}{
					"conditions": map[string]interface{}{
						"lastProbeTime":      "null",
						"lastTransitionTime": "2021-01-19T13:09:14Z",
						"status":             "True",
						"type":               "Initialized",
					},
					"containerStatuses": map[string]interface{}{
						"containerID": `docker://55ad0787835e874b6762ad650af3d36c1`,
						"image":       "redis:5.0.4",
					},
				},
			},

			newRes: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations":       map[string]interface{}{},
					"creationTimestamp": "2021-01-09T12:37:26Z",
					"labels":            map[string]interface{}{},
					"managedFields": map[string]interface{}{
						"apiVersion": "v1",
						"fieldsType": "FieldsV1",
					},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
				"status": map[string]interface{}{
					"conditions": map[string]interface{}{
						"lastProbeTime":      "null",
						"lastTransitionTime": "2021-01-19T13:09:14Z",
						"status":             "True",
						"type":               "Initialized",
					},
					"containerStatuses": map[string]interface{}{
						"containerID": `docker://55ad0787835e874b6762ad650af3d36c1`,
						"image":       "redis:5.0.4",
					},
				},
			},

			expectedObj: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels":      map[string]interface{}{},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
			},

			expectedNewRes: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
					"labels":      map[string]interface{}{},
				},
				"spec": map[string]interface{}{
					"containers": map[string]interface{}{
						"image":           "redis:5.0.4",
						"imagePullPolicy": "IfNotPresent",
						"name":            "redis",
					},
				},
				"status": map[string]interface{}{
					"conditions": map[string]interface{}{
						"lastProbeTime":      "null",
						"lastTransitionTime": "2021-01-19T13:09:14Z",
						"status":             "True",
						"type":               "Initialized",
					},
					"containerStatuses": map[string]interface{}{
						"containerID": `docker://55ad0787835e874b6762ad650af3d36c1`,
						"image":       "redis:5.0.4",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		o, n := stripNonPolicyFields(tc.obj, tc.newRes, logr.Discard())
		assert.Assert(t, reflect.DeepEqual(tc.expectedObj, o))
		assert.Assert(t, reflect.DeepEqual(tc.expectedNewRes, n))
	}

}
