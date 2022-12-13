package resource

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policycache"
	"gotest.tools/assert"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var policyCheckLabel = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	   "name": "check-label-app"
	},
	"spec": {
	   "validationFailureAction": "audit",
	   "rules": [
		  {
			 "name": "check-label-app",
			 "match": {
				"resources": {
				   "kinds": [
					  "Pod"
				   ]
				}
			 },
			 "validate": {
				"message": "The label 'app' is required.",
				"pattern": {
					"metadata": {
						"labels": {
							"app": "?*"
						}
					}
				}
			}
		  }
	   ]
	}
 }
`

var policyInvalid = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	   "name": "check-label-app"
	},
	"spec": {
	   "validationFailureAction": "audit",
	   "rules": [
		  {
			 "name": "check-label-app",
			 "match": {
				"resources": {
				   "kinds": [
					  "Pod"
				   ]
				}
			 },
			 "validate": {
				"message": "The label 'app' is required.",
				"pattern": {
					"metadata": {
						"labels": {
							"app": "{{ invalid-jmespath }}"
						}
					}
				}
			}
		  }
	   ]
	}
 }
`

var policyVerifySignature = `
{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "check-image",
        "annotations": {
            "pod-policies.kyverno.io/autogen-controllers": "none"
        }
    },
    "spec": {
        "validationFailureAction": "enforce",
        "background": false,
        "webhookTimeoutSeconds": 30,
        "failurePolicy": "Fail",
        "rules": [
            {
                "name": "check-signature",
                "match": {
                    "resources": {
                        "kinds": [
                            "Pod"
                        ]
                    }
                },
                "verifyImages": [
                    {
                        "imageReferences": [
                            "*"
                        ],
                        "attestors": [
                            {
                                "entries": [
                                    {
                                        "keys": {
                                            "publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                ]
            }
        ]
    }
}
`

var policyMutateAndVerify = `
{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "disallow-unsigned-images"
    },
    "spec": {
        "validationFailureAction": "enforce",
        "background": false,
        "rules": [
            {
                "name": "replace-image-registry",
                "match": {
                    "any": [
                        {
                            "resources": {
                                "kinds": [
                                    "Pod"
                                ]
                            }
                        }
                    ]
                },
                "mutate": {
                    "foreach": [
                        {
                            "list": "request.object.spec.containers",
                            "patchStrategicMerge": {
                                "spec": {
                                    "containers": [
                                        {
                                            "name": "{{ element.name }}",
                                            "image": "{{ regex_replace_all_literal('.*(.*)/', '{{element.image}}', 'ghcr.io/kyverno/' )}}"
                                        }
                                    ]
                                }
                            }
                        }
                    ]
                }
            },
            {
                "name": "disallow-unsigned-images-rule",
                "match": {
                    "any": [
                        {
                            "resources": {
                                "kinds": [
                                    "Pod"
                                ]
                            }
                        }
                    ]
                },
                "verifyImages": [
                    {
                        "imageReferences": [
                            "*"
                        ],
                        "verifyDigest": false,
                        "required": null,
                        "mutateDigest": false,
                        "attestors": [
                            {
                                "count": 1,
                                "entries": [
                                    {
                                        "keys": {
                                            "publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                ]
            }
        ]
    }
}
`

var resourceMutateAndVerify = `{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "labels": {
            "run": "rewrite"
        },
        "name": "rewrite"
    },
    "spec": {
        "containers": [
            {
                "image": "test-verify-image:signed",
                "name": "rewrite",
                "resources": {}
            }
        ],
        "dnsPolicy": "ClusterFirst",
        "restartPolicy": "OnFailure"
    }
}
`

var pod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
	   "name": "test-pod",
	   "namespace": ""
	},
	"spec": {
	   "containers": [
		  {
			 "name": "nginx",
			 "image": "nginx:latest"
		  }
	   ]
	}
 }
`

func Test_AdmissionResponseValid(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.WithName("Test_AdmissionResponseValid")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers := NewFakeHandlers(ctx, policyCache)

	var validPolicy kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policyCheckLabel), &validPolicy)
	assert.NilError(t, err)

	key := makeKey(&validPolicy)
	subresourceGVKToKind := make(map[string]string)
	policyCache.Set(key, &validPolicy, subresourceGVKToKind)

	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Pod"},
		Object: runtime.RawExtension{
			Raw: []byte(pod),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	response := handlers.Mutate(ctx, logger, request, "", time.Now())
	assert.Assert(t, response != nil)
	assert.Equal(t, response.Allowed, true)

	response = handlers.Validate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, true)
	assert.Equal(t, len(response.Warnings), 0)

	validPolicy.Spec.ValidationFailureAction = "Enforce"
	policyCache.Set(key, &validPolicy, subresourceGVKToKind)

	response = handlers.Validate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	policyCache.Unset(key)
}

func Test_AdmissionResponseInvalid(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.WithName("Test_AdmissionResponseInvalid")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers := NewFakeHandlers(ctx, policyCache)

	var invalidPolicy kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policyInvalid), &invalidPolicy)
	assert.NilError(t, err)

	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Pod"},
		Object: runtime.RawExtension{
			Raw: []byte(pod),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	keyInvalid := makeKey(&invalidPolicy)
	invalidPolicy.Spec.ValidationFailureAction = "Enforce"
	subresourceGVKToKind := make(map[string]string)
	policyCache.Set(keyInvalid, &invalidPolicy, subresourceGVKToKind)

	response := handlers.Validate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	var ignore kyverno.FailurePolicyType = kyverno.Ignore
	invalidPolicy.Spec.FailurePolicy = &ignore
	policyCache.Set(keyInvalid, &invalidPolicy, subresourceGVKToKind)

	response = handlers.Validate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, true)
	assert.Equal(t, len(response.Warnings), 1)
}

func Test_ImageVerify(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.WithName("Test_ImageVerify")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers := NewFakeHandlers(ctx, policyCache)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policyVerifySignature), &policy)
	assert.NilError(t, err)

	key := makeKey(&policy)
	subresourceGVKToKind := make(map[string]string)
	policyCache.Set(key, &policy, subresourceGVKToKind)

	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Pod"},
		Object: runtime.RawExtension{
			Raw: []byte(pod),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	policy.Spec.ValidationFailureAction = "Enforce"
	policyCache.Set(key, &policy, subresourceGVKToKind)

	response := handlers.Mutate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	var ignore kyverno.FailurePolicyType = kyverno.Ignore
	policy.Spec.FailurePolicy = &ignore
	policyCache.Set(key, &policy, subresourceGVKToKind)

	response = handlers.Mutate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)
}

func Test_MutateAndVerify(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.WithName("Test_MutateAndVerify")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers := NewFakeHandlers(ctx, policyCache)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policyMutateAndVerify), &policy)
	assert.NilError(t, err)

	key := makeKey(&policy)
	policyCache.Set(key, &policy, make(map[string]string))

	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Pod"},
		Object: runtime.RawExtension{
			Raw: []byte(resourceMutateAndVerify),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	response := handlers.Mutate(ctx, logger, request, "", time.Now())
	assert.Equal(t, response.Allowed, true)
	assert.Equal(t, len(response.Warnings), 0)
}

func makeKey(policy kyverno.PolicyInterface) string {
	name := policy.GetName()
	namespace := policy.GetNamespace()
	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}
