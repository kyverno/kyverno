package validation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
	v1 "k8s.io/api/admission/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var jp = jmespath.New(config.NewDefaultConfiguration(false))

var test_policy = `{}`

var signed_resource = `{
	"apiVersion": "v1",
	"data": {
		"comment": "comment1",
		"key1": "val1",
		"key2": "val2"
	},
	"kind": "ConfigMap",
	"metadata": {
		"annotations": {
			"cosign.sigstore.dev/message": "H4sIAAAAAAAA/wD8AAP/H4sIAAAAAAAA/+zRu27DIBQGYGaeghdIOVzsNKydu1Vdq9MYW5Y54AKJmjx9leuYsVUlf8vPz2U4Qu4xyz6Fzuci41EqeJ7C19DpGj+BDNFHMGS/LQDAEOWb3Caasy9ljMOqYl4Nx2azMQBGyYI0B7/a0tMBKbC709vW2nOu2+acoC/9RDVrpqyGprXQasPAQKvWTAD7BbtSMTOAPO269OBeqdj3D86vs9zzn8B5fPe5jCk6sVd8GmPnxEuK/Ti84szJV+ywouNCRCTvxP2T+W1/8gflxB6DuhR9LpoLsU1EPlZ3Wyj+1+MuFovF4uonAAD//3weCWIACAAAAQAA//+Nc9ey/AAAAA==",
			"cosign.sigstore.dev/signature": "MEYCIQCTNFfObr0DiBCbDYEq0clxRw0FeoY35LhEiIFrGU7bZAIhAJR7AEYHIXkCPGlPIXA8ao0L99s3RWAjjzoxwcvOfmeT"
		},
		"name": "sample-cm",
		"namespace": "sample-ns"
	}
}`

var signed_adreq = `{
    "uid": "2529b894-5fca-4df9-a92b-7110f42bfa09",
    "kind": {
        "group": "",
        "version": "v1",
        "kind": "ConfigMap"
    },
    "resource": {
        "group": "",
        "version": "v1",
        "resource": "configmaps"
    },
    "requestKind": {
        "group": "",
        "version": "v1",
        "kind": "ConfigMap"
    },
    "requestResource": {
        "group": "",
        "version": "v1",
        "resource": "configmaps"
    },
    "name": "sample-cm",
    "namespace": "sample-ns",
    "operation": "CREATE",
    "userInfo": {
        "username": "kubernetes-admin",
        "groups": [
            "system:masters",
            "system:authenticated"
        ]
    },
    "object": {
        "apiVersion": "v1",
        "data": {
            "comment": "comment1",
            "key1": "val1",
            "key2": "val2"
        },
        "kind": "ConfigMap",
        "metadata": {
            "annotations": {
                "cosign.sigstore.dev/message": "H4sIAAAAAAAA/wD8AAP/H4sIAAAAAAAA/+zRu27DIBQGYGaeghdIOVzsNKydu1Vdq9MYW5Y54AKJmjx9leuYsVUlf8vPz2U4Qu4xyz6Fzuci41EqeJ7C19DpGj+BDNFHMGS/LQDAEOWb3Caasy9ljMOqYl4Nx2azMQBGyYI0B7/a0tMBKbC709vW2nOu2+acoC/9RDVrpqyGprXQasPAQKvWTAD7BbtSMTOAPO269OBeqdj3D86vs9zzn8B5fPe5jCk6sVd8GmPnxEuK/Ti84szJV+ywouNCRCTvxP2T+W1/8gflxB6DuhR9LpoLsU1EPlZ3Wyj+1+MuFovF4uonAAD//3weCWIACAAAAQAA//+Nc9ey/AAAAA==",
                "cosign.sigstore.dev/signature": "MEYCIQCTNFfObr0DiBCbDYEq0clxRw0FeoY35LhEiIFrGU7bZAIhAJR7AEYHIXkCPGlPIXA8ao0L99s3RWAjjzoxwcvOfmeT"
            },
            "creationTimestamp": "2022-03-04T07:43:10Z",
            "managedFields": [
                {
                    "apiVersion": "v1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {
                        "f:data": {
                            ".": {},
                            "f:comment": {},
                            "f:key1": {},
                            "f:key2": {}
                        },
                        "f:metadata": {
                            "f:annotations": {
                                ".": {},
                                "f:integrityshield.io/message": {},
                                "f:integrityshield.io/signature": {}
                            }
                        }
                    },
                    "manager": "oc",
                    "operation": "Update",
                    "time": "2022-03-04T07:43:10Z"
                }
            ],
            "name": "sample-cm",
            "namespace": "sample-ns",
            "uid": "44725451-0fd5-47ec-98a1-f53f938e9b4d"
        }
    },
    "oldObject": null,
    "dryRun": false,
    "options": {
        "apiVersion": "meta.k8s.io/v1",
        "kind": "CreateOptions"
    }
}`

var unsigned_resource = `{
	"apiVersion": "v1",
	"data": {
		"comment": "comment1",
		"key1": "val1",
		"key2": "val2"
	},
	"kind": "ConfigMap",
	"metadata": {
		"name": "sample-cm",
		"namespace": "sample-ns"
	}
}`

var unsigned_adreq = `{
    "uid": "2529b894-5fca-4df9-a92b-7110f42bfa09",
    "kind": {
        "group": "",
        "version": "v1",
        "kind": "ConfigMap"
    },
    "resource": {
        "group": "",
        "version": "v1",
        "resource": "configmaps"
    },
    "requestKind": {
        "group": "",
        "version": "v1",
        "kind": "ConfigMap"
    },
    "requestResource": {
        "group": "",
        "version": "v1",
        "resource": "configmaps"
    },
    "name": "sample-cm",
    "namespace": "sample-ns",
    "operation": "CREATE",
    "userInfo": {
        "username": "kubernetes-admin",
        "groups": [
            "system:masters",
            "system:authenticated"
        ]
    },
    "object": {
        "apiVersion": "v1",
        "data": {
            "comment": "comment1",
            "key1": "val1",
            "key2": "val2"
        },
        "kind": "ConfigMap",
        "metadata": {
            "creationTimestamp": "2022-03-04T07:43:10Z",
            "managedFields": [
                {
                    "apiVersion": "v1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {
                        "f:data": {
                            ".": {},
                            "f:comment": {},
                            "f:key1": {},
                            "f:key2": {}
                        },
                        "f:metadata": {
                            "f:annotations": {
                                ".": {},
                                "f:integrityshield.io/message": {},
                                "f:integrityshield.io/signature": {}
                            }
                        }
                    },
                    "manager": "oc",
                    "operation": "Update",
                    "time": "2022-03-04T07:43:10Z"
                }
            ],
            "name": "sample-cm",
            "namespace": "sample-ns",
            "uid": "44725451-0fd5-47ec-98a1-f53f938e9b4d"
        }
    },
    "oldObject": null,
    "dryRun": false,
    "options": {
        "apiVersion": "meta.k8s.io/v1",
        "kind": "CreateOptions"
    }
}`

var invalid_resource = `{
	"apiVersion": "v1",
	"data": {
		"comment": "comment1",
		"key1": "val1",
		"key2": "val2",
		"key3": "val3"
	},
	"kind": "ConfigMap",
	"metadata": {
		"name": "sample-cm",
		"namespace": "sample-ns",
        "annotations": {
            "cosign.sigstore.dev/message": "H4sIAAAAAAAA/wAIAff+H4sIAAAAAAAA/+yQu07DMBSGM+cp/ALB97T1ysyGWNHBcYKVHDuy3UL79Ii0ha0jCJFv+S/+Pdj0AIn2cepcyvSoqVZb+Rwwb076pStod+M7onx7ZYyxIdBHaiPOyeXsw9AUSM1w2raca7UTdB98aYrLpbF4dwScqjOfd1ulFt20elEmznmxTFdcCS0ka7USFZNcKlkRVv0A+1wgVYylcd/FG7tcoO9vnF/e8qV/BJj9k0vZx2DIgdejD50h9zH0fniAuUZXoIMCpiYkADpDMuA8ucbipckz2O865Po6txHRhWKuhteEjO7IDTnAdAliCeK3P2FlZWXlH/IRAAD//2efbt8ACAAAAQAA//91Gk76CAEAAA==",
            "cosign.sigstore.dev/signature": "MEQCIFrMmrKUOAzbp0/5oe7TEotfqbAH9L5ao6iaIOlDZQCOAiAYcUnBFbn4GqgayTtcLN+Yi/2hH6hFz4MBUydO6DREAQ=="
        }
	}
}`

var invalid_adreq = `{
    "clusterRoles": null,
    "dryRun": false,
    "kind": {
        "group": "",
        "kind": "ConfigMap",
        "version": "v1"
    },
    "name": "sample-cm",
    "namespace": "sample-ns",
    "object": {
        "apiVersion": "v1",
        "data": {
            "comment": "comment1",
            "key1": "val1",
            "key2": "val2",
            "key3": "val3"
        },
        "kind": "ConfigMap",
        "metadata": {
            "annotations": {
                "cosign.sigstore.dev/message": "H4sIAAAAAAAA/wAIAff+H4sIAAAAAAAA/+yQu07DMBSGM+cp/ALB97T1ysyGWNHBcYKVHDuy3UL79Ii0ha0jCJFv+S/+Pdj0AIn2cepcyvSoqVZb+Rwwb076pStod+M7onx7ZYyxIdBHaiPOyeXsw9AUSM1w2raca7UTdB98aYrLpbF4dwScqjOfd1ulFt20elEmznmxTFdcCS0ka7USFZNcKlkRVv0A+1wgVYylcd/FG7tcoO9vnF/e8qV/BJj9k0vZx2DIgdejD50h9zH0fniAuUZXoIMCpiYkADpDMuA8ucbipckz2O865Po6txHRhWKuhteEjO7IDTnAdAliCeK3P2FlZWXlH/IRAAD//2efbt8ACAAAAQAA//91Gk76CAEAAA==",
                "cosign.sigstore.dev/signature": "MEQCIFrMmrKUOAzbp0/5oe7TEotfqbAH9L5ao6iaIOlDZQCOAiAYcUnBFbn4GqgayTtcLN+Yi/2hH6hFz4MBUydO6DREAQ=="
            },
            "creationTimestamp": "2022-06-15T08:11:51Z",
            "managedFields": [
                {
                    "apiVersion": "v1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {
                        "f:data": {
                            ".": {},
                            "f:comment": {},
                            "f:key1": {},
                            "f:key2": {},
                            "f:key3": {}
                        },
                        "f:metadata": {
                            "f:annotations": {
                                ".": {},
                                "f:cosign.sigstore.dev/message": {},
                                "f:cosign.sigstore.dev/signature": {}
                            }
                        }
                    },
                    "manager": "kubectl-create",
                    "operation": "Update",
                    "time": "2022-06-15T08:11:51Z"
                }
            ],
            "name": "sample-cm",
            "namespace": "sample-ns",
            "uid": "5b4e13e5-898d-4fd2-95a9-8d7a95f3b469"
        }
    },
    "oldObject": null,
    "operation": "CREATE",
    "options": {
        "apiVersion": "meta.k8s.io/v1",
        "fieldManager": "kubectl-create",
        "fieldValidation": "Strict",
        "kind": "CreateOptions"
    },
    "requestKind": {
        "group": "",
        "kind": "ConfigMap",
        "version": "v1"
    },
    "requestResource": {
        "group": "",
        "resource": "configmaps",
        "version": "v1"
    },
    "resource": {
        "group": "",
        "resource": "configmaps",
        "version": "v1"
    },
    "roles": null,
    "uid": "eecf91e9-4bba-420a-8094-261c8099a04c",
    "userInfo": {
        "groups": [
            "system:masters",
            "system:authenticated"
        ],
        "username": "kubernetes-admin"
    }
}`

var multi_sig_resource = `{
    "apiVersion": "v1",
    "kind": "Service",
    "metadata": {
      "annotations": {
        "cosign.sigstore.dev/message": "H4sIAAAAAAAA/yyKzQrCQAwG7/sU3wsUFMGf3MSzUFC8h22QxXY3JKHg20sXb8PMsJaXmJdWCes+fUqdCA+xtWRJiwRPHEwJqLwIIcRj8H92lbwlbRa+wdCRcN4lAFBr0XKbCc/b2E2wvSXGPl0Op2MCXGbJ0Yz6wKqE+/eqmn4BAAD//3vEXUSaAAAA",
        "cosign.sigstore.dev/signature": "MEUCIQDpI/7Ncl8iJJ/Kc8JlL5FLbePZprMSRRjvXYlaybjU2wIgaPYh93JMerk2L+vTOwQ4pYlZ43Eq86QnQ8wuKPXmnWE="
      },
      "name": "test-service"
    },
    "spec": {
      "ports": [
        {
          "port": 80,
          "protocol": "TCP",
          "targetPort": 9376
        }
      ],
      "selector": {
        "app": "MyApp"
      }
    }
  }`

var multi_sig_adreq = `{
    "clusterRoles": null,
    "dryRun": false,
    "kind": {
        "group": "",
        "kind": "Service",
        "version": "v1"
    },
    "name": "test-service",
    "namespace": "test-ns",
    "object": {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {
            "annotations": {
                "cosign.sigstore.dev/message": "H4sIAAAAAAAA/yyKzQrCQAwG7/sU3wsUFMGf3MSzUFC8h22QxXY3JKHg20sXb8PMsJaXmJdWCes+fUqdCA+xtWRJiwRPHEwJqLwIIcRj8H92lbwlbRa+wdCRcN4lAFBr0XKbCc/b2E2wvSXGPl0Op2MCXGbJ0Yz6wKqE+/eqmn4BAAD//3vEXUSaAAAA",
                "cosign.sigstore.dev/signature": "MEUCIQDpI/7Ncl8iJJ/Kc8JlL5FLbePZprMSRRjvXYlaybjU2wIgaPYh93JMerk2L+vTOwQ4pYlZ43Eq86QnQ8wuKPXmnWE="
            },
            "creationTimestamp": "2022-06-16T07:15:34Z",
            "managedFields": [
                {
                    "apiVersion": "v1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {
                        "f:metadata": {
                            "f:annotations": {
                                ".": {},
                                "f:cosign.sigstore.dev/message": {},
                                "f:cosign.sigstore.dev/signature": {}
                            }
                        },
                        "f:spec": {
                            "f:internalTrafficPolicy": {},
                            "f:ports": {
                                ".": {},
                                "k:{\"port\":80,\"protocol\":\"TCP\"}": {
                                    ".": {},
                                    "f:port": {},
                                    "f:protocol": {},
                                    "f:targetPort": {}
                                }
                            },
                            "f:selector": {},
                            "f:sessionAffinity": {},
                            "f:type": {}
                        }
                    },
                    "manager": "kubectl-create",
                    "operation": "Update",
                    "time": "2022-06-16T07:15:34Z"
                }
            ],
            "name": "test-service",
            "namespace": "test-ns",
            "uid": "7011a06e-50cb-46b8-bf88-40582294d2b2"
        },
        "spec": {
            "clusterIP": "10.96.122.111",
            "clusterIPs": [
                "10.96.122.111"
            ],
            "internalTrafficPolicy": "Cluster",
            "ipFamilies": [
                "IPv4"
            ],
            "ipFamilyPolicy": "SingleStack",
            "ports": [
                {
                    "port": 80,
                    "protocol": "TCP",
                    "targetPort": 9376
                }
            ],
            "selector": {
                "app": "MyApp"
            },
            "sessionAffinity": "None",
            "type": "ClusterIP"
        },
        "status": {
            "loadBalancer": {}
        }
    },
    "oldObject": null,
    "operation": "CREATE",
    "options": {
        "apiVersion": "meta.k8s.io/v1",
        "fieldManager": "kubectl-create",
        "fieldValidation": "Strict",
        "kind": "CreateOptions"
    },
    "requestKind": {
        "group": "",
        "kind": "Service",
        "version": "v1"
    },
    "requestResource": {
        "group": "",
        "resource": "services",
        "version": "v1"
    },
    "resource": {
        "group": "",
        "resource": "services",
        "version": "v1"
    },
    "roles": null,
    "uid": "cb2aed81-39ca-43c0-b565-0621b9a5d38c",
    "userInfo": {
        "groups": [
            "system:masters",
            "system:authenticated"
        ],
        "username": "kubernetes-admin"
    }
}`

var multi_sig2_resource = `{
    "apiVersion": "v1",
    "kind": "Service",
    "metadata": {
      "annotations": {
        "cosign.sigstore.dev/message": "H4sIAAAAAAAA/yyKzQrCQAwG7/sU3wsUFMGf3MSzUFC8h22QxXY3JKHg20sXb8PMsJaXmJdWCes+fUqdCA+xtWRJiwRPHEwJqLwIIcRj8H92lbwlbRa+wdCRcN4lAFBr0XKbCc/b2E2wvSXGPl0Op2MCXGbJ0Yz6wKqE+/eqmn4BAAD//3vEXUSaAAAA",
        "cosign.sigstore.dev/signature": "MEUCIQDpI/7Ncl8iJJ/Kc8JlL5FLbePZprMSRRjvXYlaybjU2wIgaPYh93JMerk2L+vTOwQ4pYlZ43Eq86QnQ8wuKPXmnWE=",
        "cosign.sigstore.dev/signature_1": "MEMCICxbOY2HKophxyUVxhBOAJo+kt+WYleDttBCVFrmA/7PAh8lgr5u3d2rFM8gSTBEYgzRzXIwAZEByrpq0SVRwqq7"
      },
      "name": "test-service"
    },
    "spec": {
      "ports": [
        {
          "port": 80,
          "protocol": "TCP",
          "targetPort": 9376
        }
      ],
      "selector": {
        "app": "MyApp"
      }
    }
  }`

var multi_sig2_adreq = `{
    "clusterRoles": null,
    "dryRun": false,
    "kind": {
        "group": "",
        "kind": "Service",
        "version": "v1"
    },
    "name": "test-service",
    "namespace": "test-ns",
    "object": {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {
            "annotations": {
                "cosign.sigstore.dev/message": "H4sIAAAAAAAA/yyKzQrCQAwG7/sU3wsUFMGf3MSzUFC8h22QxXY3JKHg20sXb8PMsJaXmJdWCes+fUqdCA+xtWRJiwRPHEwJqLwIIcRj8H92lbwlbRa+wdCRcN4lAFBr0XKbCc/b2E2wvSXGPl0Op2MCXGbJ0Yz6wKqE+/eqmn4BAAD//3vEXUSaAAAA",
                "cosign.sigstore.dev/signature": "MEUCIQDpI/7Ncl8iJJ/Kc8JlL5FLbePZprMSRRjvXYlaybjU2wIgaPYh93JMerk2L+vTOwQ4pYlZ43Eq86QnQ8wuKPXmnWE=",
                "cosign.sigstore.dev/signature_1": "MEMCICxbOY2HKophxyUVxhBOAJo+kt+WYleDttBCVFrmA/7PAh8lgr5u3d2rFM8gSTBEYgzRzXIwAZEByrpq0SVRwqq7"
            },
            "creationTimestamp": "2022-06-16T07:16:23Z",
            "managedFields": [
                {
                    "apiVersion": "v1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {
                        "f:metadata": {
                            "f:annotations": {
                                ".": {},
                                "f:cosign.sigstore.dev/message": {},
                                "f:cosign.sigstore.dev/signature": {},
                                "f:cosign.sigstore.dev/signature_1": {}
                            }
                        },
                        "f:spec": {
                            "f:internalTrafficPolicy": {},
                            "f:ports": {
                                ".": {},
                                "k:{\"port\":80,\"protocol\":\"TCP\"}": {
                                    ".": {},
                                    "f:port": {},
                                    "f:protocol": {},
                                    "f:targetPort": {}
                                }
                            },
                            "f:selector": {},
                            "f:sessionAffinity": {},
                            "f:type": {}
                        }
                    },
                    "manager": "kubectl-create",
                    "operation": "Update",
                    "time": "2022-06-16T07:16:23Z"
                }
            ],
            "name": "test-service",
            "namespace": "test-ns",
            "uid": "3e146f89-bd8a-4738-a7d4-41c5872d18ad"
        },
        "spec": {
            "clusterIP": "10.96.223.193",
            "clusterIPs": [
                "10.96.223.193"
            ],
            "internalTrafficPolicy": "Cluster",
            "ipFamilies": [
                "IPv4"
            ],
            "ipFamilyPolicy": "SingleStack",
            "ports": [
                {
                    "port": 80,
                    "protocol": "TCP",
                    "targetPort": 9376
                }
            ],
            "selector": {
                "app": "MyApp"
            },
            "sessionAffinity": "None",
            "type": "ClusterIP"
        },
        "status": {
            "loadBalancer": {}
        }
    },
    "oldObject": null,
    "operation": "CREATE",
    "options": {
        "apiVersion": "meta.k8s.io/v1",
        "fieldManager": "kubectl-create",
        "fieldValidation": "Strict",
        "kind": "CreateOptions"
    },
    "requestKind": {
        "group": "",
        "kind": "Service",
        "version": "v1"
    },
    "requestResource": {
        "group": "",
        "resource": "services",
        "version": "v1"
    },
    "resource": {
        "group": "",
        "resource": "services",
        "version": "v1"
    },
    "roles": null,
    "uid": "8a2891d0-351a-4363-92e4-0c99b10e5f26",
    "userInfo": {
        "groups": [
            "system:masters",
            "system:authenticated"
        ],
        "username": "kubernetes-admin"
    }
}`

const ecdsaPub = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyQfmL5YwHbn9xrrgG3vgbU0KJxMY
BibYLJ5L4VSMvGxeMLnBGdM48w5IE//6idUPj3rscigFdHs7GDMH4LLAng==
-----END PUBLIC KEY-----`

const ecdsaPub2 = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEE8uGVnyDWPPlB7M5KOHRzxzPHtAy
FdGxexVrR4YqO1pRViKxmD9oMu4I7K/4sM51nbH65ycB2uRiDfIdRoV/+A==
-----END PUBLIC KEY-----
`

var (
	h   = validateManifestHandler{}
	cfg = config.NewDefaultConfiguration(false)
)

func Test_VerifyManifest_SignedYAML(t *testing.T) {
	policyContext := buildContext(t, kyvernov1.Create, test_policy, signed_resource, "")
	var request v1.AdmissionRequest
	_ = json.Unmarshal([]byte(signed_adreq), &request)
	policyContext.JSONContext().AddRequest(request)
	policyContext.Policy().SetName("test-policy")
	verifyRule := kyvernov1.Manifests{}
	verifyRule.Attestors = append(verifyRule.Attestors, kyvernov1.AttestorSet{
		Entries: []kyvernov1.Attestor{
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub,
				},
			},
		},
	})
	logger := logr.Discard()
	verified, _, err := h.verifyManifest(context.TODO(), logger, policyContext, verifyRule)
	assert.NilError(t, err)
	assert.Equal(t, verified, true)
}

func Test_VerifyManifest_UnsignedYAML(t *testing.T) {
	policyContext := buildContext(t, kyvernov1.Create, test_policy, unsigned_resource, "")
	var request v1.AdmissionRequest
	_ = json.Unmarshal([]byte(unsigned_adreq), &request)
	policyContext.JSONContext().AddRequest(request)
	policyContext.Policy().SetName("test-policy")
	verifyRule := kyvernov1.Manifests{}
	verifyRule.Attestors = append(verifyRule.Attestors, kyvernov1.AttestorSet{
		Entries: []kyvernov1.Attestor{
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub,
				},
			},
		},
	})
	logger := logr.Discard()
	verified, _, err := h.verifyManifest(context.TODO(), logger, policyContext, verifyRule)
	assert.NilError(t, err)
	assert.Equal(t, verified, false)
}

func Test_VerifyManifest_InvalidYAML(t *testing.T) {
	policyContext := buildContext(t, kyvernov1.Create, test_policy, invalid_resource, "")
	var request v1.AdmissionRequest
	_ = json.Unmarshal([]byte(invalid_adreq), &request)
	policyContext.JSONContext().AddRequest(request)
	policyContext.Policy().SetName("test-policy")
	verifyRule := kyvernov1.Manifests{}
	verifyRule.Attestors = append(verifyRule.Attestors, kyvernov1.AttestorSet{
		Entries: []kyvernov1.Attestor{
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub,
				},
			},
		},
	})
	logger := logr.Discard()
	verified, _, err := h.verifyManifest(context.TODO(), logger, policyContext, verifyRule)
	assert.NilError(t, err)
	assert.Equal(t, verified, false)
}

func Test_VerifyManifest_MustAll_InvalidYAML(t *testing.T) {
	policyContext := buildContext(t, kyvernov1.Create, test_policy, multi_sig_resource, "")
	var request v1.AdmissionRequest
	_ = json.Unmarshal([]byte(multi_sig_adreq), &request)
	policyContext.JSONContext().AddRequest(request)
	policyContext.Policy().SetName("test-policy")
	verifyRule := kyvernov1.Manifests{}
	verifyRule.Attestors = append(verifyRule.Attestors, kyvernov1.AttestorSet{
		Entries: []kyvernov1.Attestor{
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub,
				},
			},
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub2,
				},
			},
		},
	})
	logger := logr.Discard()
	verified, _, err := h.verifyManifest(context.TODO(), logger, policyContext, verifyRule)
	errMsg := `.attestors[0].entries[1].keys: failed to verify signature: verification failed for 1 signature. all trials: ["[publickey 1/1] [signature 1/1] error: cosign.VerifyBlobCmd.Exec() returned an error: invalid signature when validating ASN.1 encoded signature"]`
	assert.Error(t, err, errMsg)
	assert.Equal(t, verified, false)
}

func Test_VerifyManifest_MustAll_ValidYAML(t *testing.T) {
	policyContext := buildContext(t, kyvernov1.Create, test_policy, multi_sig2_resource, "")
	var request v1.AdmissionRequest
	_ = json.Unmarshal([]byte(multi_sig2_adreq), &request)
	policyContext.JSONContext().AddRequest(request)
	policyContext.Policy().SetName("test-policy")
	verifyRule := kyvernov1.Manifests{}
	count := 3
	verifyRule.Attestors = append(verifyRule.Attestors, kyvernov1.AttestorSet{
		Count: &count,
		Entries: []kyvernov1.Attestor{
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub,
				},
			},
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub2,
				},
			},
			{
				Attestor: &apiextv1.JSON{Raw: []byte(`{"entries":[{"keys":{"publicKeys":"-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyQfmL5YwHbn9xrrgG3vgbU0KJxMY\nBibYLJ5L4VSMvGxeMLnBGdM48w5IE//6idUPj3rscigFdHs7GDMH4LLAng==\n-----END PUBLIC KEY-----    "}}]}`)},
			},
		},
	})
	logger := logr.Discard()
	verified, _, err := h.verifyManifest(context.TODO(), logger, policyContext, verifyRule)
	assert.NilError(t, err)
	assert.Equal(t, verified, true)
}

func Test_VerifyManifest_AtLeastOne(t *testing.T) {
	policyContext := buildContext(t, kyvernov1.Create, test_policy, multi_sig_resource, "")
	var request v1.AdmissionRequest
	_ = json.Unmarshal([]byte(multi_sig_adreq), &request)
	policyContext.JSONContext().AddRequest(request)
	policyContext.Policy().SetName("test-policy")
	verifyRule := kyvernov1.Manifests{}
	count := 1
	verifyRule.Attestors = append(verifyRule.Attestors, kyvernov1.AttestorSet{
		Count: &count,
		Entries: []kyvernov1.Attestor{
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub,
				},
			},
			{
				Keys: &kyvernov1.StaticKeyAttestor{
					PublicKeys: ecdsaPub2,
				},
			},
		},
	})
	logger := logr.Discard()
	verified, _, err := h.verifyManifest(context.TODO(), logger, policyContext, verifyRule)
	assert.NilError(t, err)
	assert.Equal(t, verified, true)
}

func buildContext(t *testing.T, operation kyvernov1.AdmissionOperation, policy, resource string, oldResource string) engineapi.PolicyContext {
	var cpol kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(policy), &cpol)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(resource))
	assert.NilError(t, err)

	policyContext, err := policycontext.NewPolicyContext(
		jp,
		*resourceUnstructured,
		operation,
		nil,
		cfg,
	)
	assert.NilError(t, err)

	policyContext = policyContext.
		WithPolicy(&cpol).
		WithNewResource(*resourceUnstructured)

	if oldResource != "" {
		oldResourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(oldResource))
		assert.NilError(t, err)

		err = enginecontext.AddOldResource(policyContext.JSONContext(), []byte(oldResource))
		assert.NilError(t, err)

		policyContext = policyContext.WithOldResource(*oldResourceUnstructured)
	}

	return policyContext
}
