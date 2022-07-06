package resource

import (
	"context"
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policycache"
	"gotest.tools/assert"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	log "sigs.k8s.io/controller-runtime/pkg/log"

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
	logger := log.Log.WithName("Test_AdmissionResponseValid")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers := NewFakeHandlers(ctx, policyCache)

	var validPolicy kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policyCheckLabel), &validPolicy)
	assert.NilError(t, err)

	key := makeKey(&validPolicy)
	policyCache.Set(key, &validPolicy)

	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Pod"},
		Object: runtime.RawExtension{
			Raw: []byte(pod),
		},
	}

	response := handlers.Mutate(logger, request)
	assert.Assert(t, response != nil)
	assert.Equal(t, response.Allowed, true)

	response = handlers.Validate(logger, request)
	assert.Equal(t, response.Allowed, true)
	assert.Equal(t, len(response.Warnings), 0)

	validPolicy.Spec.ValidationFailureAction = kyverno.Enforce
	policyCache.Set(key, &validPolicy)

	response = handlers.Validate(logger, request)
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	policyCache.Unset(key)
}

func Test_AdmissionResponseInvalid(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.Log.WithName("Test_AdmissionResponseInvalid")

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
	}

	keyInvalid := makeKey(&invalidPolicy)
	invalidPolicy.Spec.ValidationFailureAction = kyverno.Enforce
	policyCache.Set(keyInvalid, &invalidPolicy)

	response := handlers.Validate(logger, request)
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	var ignore kyverno.FailurePolicyType = kyverno.Ignore
	invalidPolicy.Spec.FailurePolicy = &ignore
	policyCache.Set(keyInvalid, &invalidPolicy)

	response = handlers.Validate(logger, request)
	assert.Equal(t, response.Allowed, true)
	assert.Equal(t, len(response.Warnings), 1)
}

func Test_ImageVerify(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.Log.WithName("Test_ImageVerify")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlers := NewFakeHandlers(ctx, policyCache)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policyVerifySignature), &policy)
	assert.NilError(t, err)

	key := makeKey(&policy)
	policyCache.Set(key, &policy)

	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "Pod"},
		Object: runtime.RawExtension{
			Raw: []byte(pod),
		},
	}

	policy.Spec.ValidationFailureAction = kyverno.Enforce
	policyCache.Set(key, &policy)

	response := handlers.Mutate(logger, request)
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	var ignore kyverno.FailurePolicyType = kyverno.Ignore
	policy.Spec.FailurePolicy = &ignore
	policyCache.Set(key, &policy)

	response = handlers.Mutate(logger, request)
	assert.Equal(t, response.Allowed, false)
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
