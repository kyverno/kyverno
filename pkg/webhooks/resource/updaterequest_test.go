package resource

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"gotest.tools/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	testMutateExistingPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "test-mutate-existing"
		},
		"spec": {
			"rules": [
				{
					"name": "add-label",
					"match": {
						"any": [
							{
								"resources": {
									"kinds": ["Pod"]
								}
							}
						]
					},
					"mutate": {
						"targets": [
							{
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"namespace": "default",
								"name": "test-cm"
							}
						],
						"patchStrategicMerge": {
							"metadata": {
								"labels": {
									"updated-by": "kyverno"
								}
							}
						}
					}
				}
			]
		}
	}`

	testMutateExistingPolicyWithDelete = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "test-mutate-existing-delete"
		},
		"spec": {
			"rules": [
				{
					"name": "cleanup-on-delete",
					"match": {
						"any": [
							{
								"resources": {
									"kinds": ["Pod"],
									"operations": ["DELETE"]
								}
							}
						]
					},
					"mutate": {
						"targets": [
							{
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"namespace": "default",
								"name": "test-cm"
							}
						],
						"patchStrategicMerge": {
							"metadata": {
								"labels": {
									"pod-deleted": "true"
								}
							}
						}
					}
				}
			]
		}
	}`

	testMutateExistingPolicySkipBackground = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "test-skip-background"
		},
		"spec": {
			"rules": [
				{
					"name": "add-label",
					"skipBackgroundRequests": true,
					"match": {
						"any": [
							{
								"resources": {
									"kinds": ["Pod"]
								}
							}
						]
					},
					"mutate": {
						"targets": [
							{
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"namespace": "default",
								"name": "test-cm"
							}
						],
						"patchStrategicMerge": {
							"metadata": {
								"labels": {
									"updated-by": "kyverno"
								}
							}
						}
					}
				}
			]
		}
	}`

	testGeneratePolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "test-generate"
		},
		"spec": {
			"rules": [
				{
					"name": "generate-configmap",
					"match": {
						"any": [
							{
								"resources": {
									"kinds": ["Namespace"]
								}
							}
						]
					},
					"generate": {
						"kind": "ConfigMap",
						"name": "default-config",
						"namespace": "{{request.object.metadata.name}}",
						"synchronize": true,
						"data": {
							"apiVersion": "v1",
							"kind": "ConfigMap",
							"metadata": {
								"name": "default-config"
							},
							"data": {
								"key": "value"
							}
						}
					}
				}
			]
		}
	}`

	testPod = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		},
		"spec": {
			"containers": [
				{
					"name": "nginx",
					"image": "nginx:latest"
				}
			]
		}
	}`

	testNamespace = `{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {
			"name": "test-namespace"
		}
	}`
)

func Test_handleMutateExisting(t *testing.T) {
	tests := []struct {
		name                         string
		policy                       string
		resource                     string
		operation                    admissionv1.Operation
		username                     string
		backgroundServiceAccountName string
		expectUpdateRequest          bool
	}{
		{
			name:                         "mutate existing policy with CREATE operation",
			policy:                       testMutateExistingPolicy,
			resource:                     testPod,
			operation:                    admissionv1.Create,
			username:                     "admin",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectUpdateRequest:          true,
		},
		{
			name:                         "mutate existing policy with DELETE operation without matching rule",
			policy:                       testMutateExistingPolicy,
			resource:                     testPod,
			operation:                    admissionv1.Delete,
			username:                     "admin",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectUpdateRequest:          false,
		},
		{
			name:                         "mutate existing policy with DELETE operation with matching rule",
			policy:                       testMutateExistingPolicyWithDelete,
			resource:                     testPod,
			operation:                    admissionv1.Delete,
			username:                     "admin",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectUpdateRequest:          true,
		},
		{
			name:                         "skip background requests when user is background service account",
			policy:                       testMutateExistingPolicySkipBackground,
			resource:                     testPod,
			operation:                    admissionv1.Create,
			username:                     "system:serviceaccount:kyverno:kyverno-background-controller",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectUpdateRequest:          false,
		},
		{
			name:                         "process background requests when user is not background service account",
			policy:                       testMutateExistingPolicySkipBackground,
			resource:                     testPod,
			operation:                    admissionv1.Create,
			username:                     "admin",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectUpdateRequest:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pCache := policycache.NewCache()
			h := NewFakeHandlers(ctx, pCache)
			h.backgroundServiceAccountName = tt.backgroundServiceAccountName

			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal([]byte(tt.policy), &policy)
			assert.NilError(t, err)

			var resourceObj map[string]interface{}
			err = json.Unmarshal([]byte(tt.resource), &resourceObj)
			assert.NilError(t, err)

			resourceRaw, err := json.Marshal(resourceObj)
			assert.NilError(t, err)

			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			}

			admissionReq := admissionv1.AdmissionRequest{
				UID: "test-uid",
				Kind: metav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
				Resource: metav1.GroupVersionResource{
					Group:    gvk.Group,
					Version:  gvk.Version,
					Resource: "pods",
				},
				Operation: tt.operation,
				Object: runtime.RawExtension{
					Raw: resourceRaw,
				},
				OldObject: runtime.RawExtension{
					Raw: resourceRaw,
				},
				Namespace: "default",
				UserInfo: authenticationv1.UserInfo{
					Username: tt.username,
				},
			}

			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionReq,
				GroupVersionKind: gvk,
				Roles:            nil,
				ClusterRoles:     nil,
			}

			logger := logr.Discard()
			policies := []kyvernov1.PolicyInterface{&policy}

			h.handleMutateExisting(ctx, logger, request, policies, time.Now())
		})
	}
}

func Test_handleGenerate(t *testing.T) {
	tests := []struct {
		name                         string
		policy                       string
		resource                     string
		operation                    admissionv1.Operation
		username                     string
		backgroundServiceAccountName string
		expectGeneration             bool
	}{
		{
			name:                         "generate policy with namespace creation",
			policy:                       testGeneratePolicy,
			resource:                     testNamespace,
			operation:                    admissionv1.Create,
			username:                     "admin",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectGeneration:             true,
		},
		{
			name:                         "skip generate when user is background service account",
			policy:                       testGeneratePolicy,
			resource:                     testNamespace,
			operation:                    admissionv1.Create,
			username:                     "system:serviceaccount:kyverno:kyverno-background-controller",
			backgroundServiceAccountName: "system:serviceaccount:kyverno:kyverno-background-controller",
			expectGeneration:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pCache := policycache.NewCache()
			h := NewFakeHandlers(ctx, pCache)
			h.backgroundServiceAccountName = tt.backgroundServiceAccountName

			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal([]byte(tt.policy), &policy)
			assert.NilError(t, err)

			var resourceObj map[string]interface{}
			err = json.Unmarshal([]byte(tt.resource), &resourceObj)
			assert.NilError(t, err)

			resourceRaw, err := json.Marshal(resourceObj)
			assert.NilError(t, err)

			gvk := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    resourceObj["kind"].(string),
			}

			admissionReq := admissionv1.AdmissionRequest{
				UID: "test-uid",
				Kind: metav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
				Resource: metav1.GroupVersionResource{
					Group:    gvk.Group,
					Version:  gvk.Version,
					Resource: "namespaces",
				},
				Operation: tt.operation,
				Object: runtime.RawExtension{
					Raw: resourceRaw,
				},
				Namespace: "",
				UserInfo: authenticationv1.UserInfo{
					Username: tt.username,
				},
			}

			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionReq,
				GroupVersionKind: gvk,
				Roles:            nil,
				ClusterRoles:     nil,
			}

			logger := logr.Discard()
			policies := []kyvernov1.PolicyInterface{&policy}

			h.handleGenerate(ctx, logger, request, policies, time.Now())
		})
	}
}

func Test_handleBackgroundApplies(t *testing.T) {
	ctx := context.Background()
	pCache := policycache.NewCache()
	h := NewFakeHandlers(ctx, pCache)
	h.backgroundServiceAccountName = "system:serviceaccount:kyverno:kyverno-background-controller"

	var mutatePolicy kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(testMutateExistingPolicy), &mutatePolicy)
	assert.NilError(t, err)

	var generatePolicy kyvernov1.ClusterPolicy
	err = json.Unmarshal([]byte(testGeneratePolicy), &generatePolicy)
	assert.NilError(t, err)

	var resourceObj map[string]interface{}
	err = json.Unmarshal([]byte(testPod), &resourceObj)
	assert.NilError(t, err)

	resourceRaw, err := json.Marshal(resourceObj)
	assert.NilError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	admissionReq := admissionv1.AdmissionRequest{
		UID: "test-uid",
		Kind: metav1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		Resource: metav1.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: "pods",
		},
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: resourceRaw,
		},
		Namespace: "default",
		UserInfo: authenticationv1.UserInfo{
			Username: "admin",
		},
	}

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionReq,
		GroupVersionKind: gvk,
		Roles:            nil,
		ClusterRoles:     nil,
	}

	logger := logr.Discard()
	generatePolicies := []kyvernov1.PolicyInterface{&generatePolicy}
	mutatePolicies := []kyvernov1.PolicyInterface{&mutatePolicy}

	wg := &wait.Group{}

	h.handleBackgroundApplies(ctx, logger, request, generatePolicies, mutatePolicies, time.Now(), wg)

	wg.Wait()
}

func Test_skipBackgroundRequests(t *testing.T) {
	tests := []struct {
		name                string
		policy              string
		backgroundSaDesired string
		backgroundSaActual  string
		expectedRulesCount  int
		expectedPolicyIsNil bool
	}{
		{
			name:                "policy without skipBackgroundRequests - different service accounts",
			policy:              testMutateExistingPolicy,
			backgroundSaDesired: "system:serviceaccount:kyverno:kyverno-background-controller",
			backgroundSaActual:  "admin",
			expectedRulesCount:  1,
			expectedPolicyIsNil: false,
		},
		{
			name:                "policy with skipBackgroundRequests=true - same service accounts",
			policy:              testMutateExistingPolicySkipBackground,
			backgroundSaDesired: "system:serviceaccount:kyverno:kyverno-background-controller",
			backgroundSaActual:  "system:serviceaccount:kyverno:kyverno-background-controller",
			expectedRulesCount:  0,
			expectedPolicyIsNil: true,
		},
		{
			name:                "policy with skipBackgroundRequests=true - different service accounts",
			policy:              testMutateExistingPolicySkipBackground,
			backgroundSaDesired: "system:serviceaccount:kyverno:kyverno-background-controller",
			backgroundSaActual:  "admin",
			expectedRulesCount:  1,
			expectedPolicyIsNil: false,
		},
		{
			name:                "policy without skipBackgroundRequests - same service accounts",
			policy:              testMutateExistingPolicy,
			backgroundSaDesired: "system:serviceaccount:kyverno:kyverno-background-controller",
			backgroundSaActual:  "system:serviceaccount:kyverno:kyverno-background-controller",
			expectedRulesCount:  0,
			expectedPolicyIsNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal([]byte(tt.policy), &policy)
			assert.NilError(t, err)

			logger := logr.Discard()

			result := skipBackgroundRequests(&policy, logger, tt.backgroundSaDesired, tt.backgroundSaActual)

			if tt.expectedPolicyIsNil {
				assert.Assert(t, result == nil, "expected policy to be nil")
			} else {
				assert.Assert(t, result != nil, "expected policy to not be nil")
				assert.Equal(t, len(result.GetSpec().Rules), tt.expectedRulesCount)
			}
		})
	}
}

func Test_handleMutateExisting_withDeleteOperation(t *testing.T) {
	ctx := context.Background()
	pCache := policycache.NewCache()
	h := NewFakeHandlers(ctx, pCache)
	h.backgroundServiceAccountName = "system:serviceaccount:kyverno:kyverno-background-controller"

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(testMutateExistingPolicy), &policy)
	assert.NilError(t, err)

	var resourceObj map[string]interface{}
	err = json.Unmarshal([]byte(testPod), &resourceObj)
	assert.NilError(t, err)

	resourceRaw, err := json.Marshal(resourceObj)
	assert.NilError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	admissionReq := admissionv1.AdmissionRequest{
		UID: "test-uid",
		Kind: metav1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		Resource: metav1.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: "pods",
		},
		Operation: admissionv1.Delete,
		OldObject: runtime.RawExtension{
			Raw: resourceRaw,
		},
		Namespace: "default",
		UserInfo: authenticationv1.UserInfo{
			Username: "admin",
		},
	}

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionReq,
		GroupVersionKind: gvk,
		Roles:            nil,
		ClusterRoles:     nil,
	}

	logger := logr.Discard()
	policies := []kyvernov1.PolicyInterface{&policy}

	h.handleMutateExisting(ctx, logger, request, policies, time.Now())

}

func Test_handleMutateExisting_noPolicyWithMutateExisting(t *testing.T) {
	ctx := context.Background()
	pCache := policycache.NewCache()
	h := NewFakeHandlers(ctx, pCache)
	h.backgroundServiceAccountName = "system:serviceaccount:kyverno:kyverno-background-controller"

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(testGeneratePolicy), &policy)
	assert.NilError(t, err)

	var resourceObj map[string]interface{}
	err = json.Unmarshal([]byte(testPod), &resourceObj)
	assert.NilError(t, err)

	resourceRaw, err := json.Marshal(resourceObj)
	assert.NilError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	admissionReq := admissionv1.AdmissionRequest{
		UID: "test-uid",
		Kind: metav1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		Resource: metav1.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: "pods",
		},
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: resourceRaw,
		},
		Namespace: "default",
		UserInfo: authenticationv1.UserInfo{
			Username: "admin",
		},
	}

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionReq,
		GroupVersionKind: gvk,
		Roles:            nil,
		ClusterRoles:     nil,
	}

	logger := logr.Discard()
	policies := []kyvernov1.PolicyInterface{&policy}

	h.handleMutateExisting(ctx, logger, request, policies, time.Now())
}
