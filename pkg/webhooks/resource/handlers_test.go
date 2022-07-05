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

func Test_AdmissionResponse(t *testing.T) {
	policyCache := policycache.NewCache()
	logger := log.Log.WithName("Test_AdmissionResponse")

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

	var invalidPolicy kyverno.ClusterPolicy
	err = json.Unmarshal([]byte(policyInvalid), &invalidPolicy)
	assert.NilError(t, err)

	keyInvalid := makeKey(&invalidPolicy)

	invalidPolicy.Spec.ValidationFailureAction = kyverno.Enforce
	policyCache.Set(keyInvalid, &invalidPolicy)
	response = handlers.Validate(logger, request)
	assert.Equal(t, response.Allowed, false)
	assert.Equal(t, len(response.Warnings), 0)

	var ignore kyverno.FailurePolicyType = kyverno.Ignore
	invalidPolicy.Spec.FailurePolicy = &ignore
	policyCache.Set(keyInvalid, &invalidPolicy)
	response = handlers.Validate(logger, request)

	assert.Equal(t, response.Allowed, true)
	assert.Equal(t, len(response.Warnings), 1)
}

func makeKey(policy kyverno.PolicyInterface) string {
	name := policy.GetName()
	namespace := policy.GetNamespace()
	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}
