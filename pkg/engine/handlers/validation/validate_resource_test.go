package validation

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_validateOldObject(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t, validateDenyPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, nil)

	ctx := context.TODO()
	resp := v.validate(ctx)
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusPass, resp.Status())

	rule2 := policyContext.Policy().GetSpec().Rules[1]
	v2 := newValidator(logr.Discard(), mockCL, policyContext, rule2, nil)
	resp2 := v2.validate(ctx)
	assert.NotNil(t, resp2 != nil)
	assert.Equal(t, api.RuleStatusFail, resp2.Status())
}

func buildTestNamespaceLabelsContext(t *testing.T, policy string, resource string, oldResource string) api.PolicyContext {
	return buildContext(t, kyvernov1.Update, policy, resource, oldResource)
}

func Test_validateOldObjectForeach(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t, validateForeachPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, nil)

	ctx := context.TODO()
	resp := v.validate(ctx)
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusSkip, resp.Status())
}

func Test_validateForEach_ElementError_NonLastElement_ReturnsError(t *testing.T) {
	// A context loader that always fails, simulating an API call timeout
	failingCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		if len(contextEntries) > 0 {
			return fmt.Errorf("simulated API call timeout")
		}
		return nil
	}

	// The resource has 2 containers — the error occurs on element 0 (not the last).
	// Before the fix, this would silently continue and return Pass.
	policyContext := buildTestNamespaceLabelsContext(t, validateForeachWithContextPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), failingCL, policyContext, rule, nil)

	ctx := context.TODO()
	resp := v.validateForEach(ctx)

	assert.NotNil(t, resp, "validateForEach should return error when a non-last element fails")
	assert.Equal(t, api.RuleStatusError, resp.Status(), "status should be Error when any element's context loading fails")
}

func Test_validateForEach_ListEvalError_ReturnsError(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	// Use a policy with an invalid list expression that will fail evaluation
	policyContext := buildTestNamespaceLabelsContext(t, validateForeachInvalidListPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, nil)

	ctx := context.TODO()
	resp := v.validateForEach(ctx)

	// Should return error response instead of nil when list evaluation fails
	assert.NotNil(t, resp, "validateForEach should return error response when list evaluation fails")
	assert.Equal(t, api.RuleStatusError, resp.Status(), "status should be Error when list evaluation fails")
}

var (
	validateDenyPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "block-label-changes"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
			  "name": "require-labels",
			  "match": {
				"all": [
				  {
					"resources": {
					  "operations": [
						"CREATE",
						"UPDATE"
					  ],
					  "kinds": [
						"Pod"
					  ]
					}
				  }
				]
			  },
			  "validate": {
			    "failureAction": "Enforce",
				"message": "The label size is required",
				"pattern": {
				  "metadata": {
					"labels": {
					  "size": "small | medium | large"
					}
				  }
				}
			  }
			},
			{
			  "name": "check-mutable-labels",
			  "match": {
				"all": [
				  {
					"resources": {
					  "operations": [
						"UPDATE"
					  ],
					  "kinds": [
						"Pod"
					  ]
					}
				  }
				]
			  },
			  "validate": {
			    "failureAction": "Enforce",
				"message": "The label size cannot be changed for a namespace",
				"deny": {
				  "conditions": {
					"all": [
					  {
						"key": "{{ request.object.metadata.labels.size || '' }}",
						"operator": "NotEquals",
						"value": "{{ request.oldObject.metadata.labels.size }}"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	}`

	validateForeachPolicy = `{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "validate-image-list"
  },
  "spec": {
    "admission": true,
    "background": true,
    "rules": [
      {
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
        "name": "check-image",
        "validate": {
    	    "failureAction": "Enforce",
		      "allowExistingViolations": true,
            "foreach": [
            {
              "deny": {
                "conditions": {
                  "all": [
                    {
                      "key": "{{ element }}",
                      "operator": "NotEquals",
                      "value": "ghcr.io"
                    }
                  ]
                }
              },
              "list": "request.object.spec.containers[].image"
            }
          ],
          "message": "images must begin with ghcr.io"
        }
      }
    ]
  }
}
	`

	resource = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "annotations": {},
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "large"
		  },
		  "name": "test"
		},
		"spec": {
			"containers": [
				{
					"image": "ghcr.io/test-webserver",
					"name": "test1",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						}
					]
				},
				{
					"image": "ghcr.io/test-webserver",
					"name": "test2",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						},
						{
							"mountPath": "/gce",
							"name": "gce"
						}
					]
				}
			],
			"volumes": [
				{
					"name": "cache-volume",
					"emptyDir": {}
				},
				{
					"name": "gce",
					"gcePersistentDisk": {}
				}
			]
		}
	}`

	oldResource = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "small"
		  },
		  "name": "test"
		},
		"spec": {
			"containers": [
				{
					"image": "ghcr.io/test-webserver",
					"name": "test1",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						}
					]
				},
				{
					"image": "ghcr.io/test-webserver",
					"name": "test2",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						},
						{
							"mountPath": "/gce",
							"name": "gce"
						}
					]
				}
			],
			"volumes": [
				{
					"name": "cache-volume",
					"emptyDir": {}
				},
				{
					"name": "gce",
					"gcePersistentDisk": {}
				}
			]
		}
	}`

	// Policy with forEach + context entries to test per-element error handling
	validateForeachWithContextPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test-foreach-context"},
		"spec": {
			"rules": [{
				"name": "check-images",
				"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
				"validate": {
					"failureAction": "Enforce",
					"foreach": [{
						"list": "request.object.spec.containers[]",
						"context": [{
							"name": "registry",
							"variable": {"value": "test"}
						}],
						"deny": {
							"conditions": {
								"all": [{
									"key": "{{ element.name }}",
									"operator": "Equals",
									"value": "blocked"
								}]
							}
						}
					}]
				}
			}]
		}
	}`

	// Policy with invalid JMESPath list expression to test error handling
	validateForeachInvalidListPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test-invalid-list"},
		"spec": {
			"rules": [{
				"name": "invalid-list-rule",
				"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
				"validate": {
					"failureAction": "Enforce",
					"foreach": [{
						"list": "invalid_jmespath_expression[",
						"deny": {"conditions": {"all": [{"key": "{{ element }}", "operator": "Equals", "value": "test"}]}}
					}]
				}
			}]
		}
	}`
)

// ── mock client + helpers for CREATE owner-chain tests ────────────────────

type mockValidateClient struct {
	resources map[string]*unstructured.Unstructured
}

func (m *mockValidateClient) GetResource(_ context.Context, _ string, kind, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	key := kind + "/" + namespace + "/" + name
	if obj, ok := m.resources[key]; ok {
		return obj, nil
	}
	return nil, fmt.Errorf("resource not found: %s", key)
}
func (m *mockValidateClient) ListResource(_ context.Context, _, _, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, nil
}
func (m *mockValidateClient) GetResources(_ context.Context, _, _, _, _, _, _ string, _ *metav1.LabelSelector) ([]api.Resource, error) {
	return nil, nil
}
func (m *mockValidateClient) GetNamespace(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Namespace, error) {
	return nil, nil
}
func (m *mockValidateClient) IsNamespaced(_, _, _ string) (bool, error) { return true, nil }
func (m *mockValidateClient) CanI(_ context.Context, _, _, _, _, _ string) (bool, string, error) {
	return true, "", nil
}
func (m *mockValidateClient) RawAbsPath(_ context.Context, _, _ string, _ io.Reader) ([]byte, error) {
	return nil, nil
}

func makeDeployObj(name, ns string, ts metav1.Time) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetKind("Deployment")
	u.SetAPIVersion("apps/v1")
	u.SetName(name)
	u.SetNamespace(ns)
	u.SetCreationTimestamp(ts)
	return u
}

func makeRSObj(name, ns string, ts metav1.Time, deployName string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetKind("ReplicaSet")
	u.SetAPIVersion("apps/v1")
	u.SetName(name)
	u.SetNamespace(ns)
	u.SetCreationTimestamp(ts)
	ctrl := true
	u.SetOwnerReferences([]metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "Deployment", Name: deployName, Controller: &ctrl},
	})
	return u
}

// allowExistingViolationsCreatePolicy: Enforce + allowExistingViolations=true
// fails Pods that have no "team" label.
// Policy creationTimestamp = 2024-06-01T00:00:00Z
var allowExistingViolationsCreatePolicy = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "require-team-label",
		"creationTimestamp": "2024-06-01T00:00:00Z"
	},
	"spec": {
		"rules": [{
			"name": "require-team",
			"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
			"validate": {
				"failureAction": "Enforce",
				"allowExistingViolations": true,
				"message": "The label team is required",
				"pattern": {"metadata": {"labels": {"team": "?*"}}}
			}
		}]
	}
}`

// podManagedNoTeamLabel: Pod with owner ref -> ReplicaSet, missing "team" label (violates policy)
var podManagedNoTeamLabel = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "default",
		"creationTimestamp": "2024-06-15T00:00:00Z",
		"ownerReferences": [{"apiVersion": "apps/v1", "kind": "ReplicaSet", "name": "test-rs", "controller": true}]
	},
	"spec": {"containers": [{"name": "c", "image": "nginx"}]}
}`

// podUnmanagedNoTeamLabel: Pod with no owner, missing "team" label (violates policy)
var podUnmanagedNoTeamLabel = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod-no-owner",
		"namespace": "default",
		"creationTimestamp": "2024-06-15T00:00:00Z"
	},
	"spec": {"containers": [{"name": "c", "image": "nginx"}]}
}`

// Test_allowExistingViolations_CREATE_OwnerPredatesPolicy:
// Root Deployment predates policy -> rule must be Skipped.
func Test_allowExistingViolations_CREATE_OwnerPredatesPolicy(t *testing.T) {
	mockCL := func(_ context.Context, _ []kyvernov1.ContextEntry, _ enginecontext.Interface) error { return nil }

	// Deployment created 2024-01-01, BEFORE policy (2024-06-01)
	client := &mockValidateClient{resources: map[string]*unstructured.Unstructured{
		"ReplicaSet/default/test-rs":     makeRSObj("test-rs", "default", metav1.Date(2024, 1, 2, 0, 0, 0, 0, metav1.Now().Location()), "test-deploy"),
		"Deployment/default/test-deploy": makeDeployObj("test-deploy", "default", metav1.Date(2024, 1, 1, 0, 0, 0, 0, metav1.Now().Location())),
	}}

	policyContext := buildContext(t, kyvernov1.Create, allowExistingViolationsCreatePolicy, podManagedNoTeamLabel, "")
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, client)

	resp := v.validate(context.TODO())
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusSkip, resp.Status(),
		"root Deployment predates policy: must be Skipped as pre-existing violation")
}

// Test_allowExistingViolations_CREATE_OwnerNewerThanPolicy:
// Root Deployment newer than policy -> enforcement must proceed (Fail).
func Test_allowExistingViolations_CREATE_OwnerNewerThanPolicy(t *testing.T) {
	mockCL := func(_ context.Context, _ []kyvernov1.ContextEntry, _ enginecontext.Interface) error { return nil }

	// Deployment created 2024-12-01, AFTER policy (2024-06-01)
	client := &mockValidateClient{resources: map[string]*unstructured.Unstructured{
		"ReplicaSet/default/test-rs":     makeRSObj("test-rs", "default", metav1.Date(2024, 12, 2, 0, 0, 0, 0, metav1.Now().Location()), "test-deploy"),
		"Deployment/default/test-deploy": makeDeployObj("test-deploy", "default", metav1.Date(2024, 12, 1, 0, 0, 0, 0, metav1.Now().Location())),
	}}

	policyContext := buildContext(t, kyvernov1.Create, allowExistingViolationsCreatePolicy, podManagedNoTeamLabel, "")
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, client)

	resp := v.validate(context.TODO())
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusFail, resp.Status(),
		"root Deployment newer than policy: enforcement must proceed")
}

// Test_allowExistingViolations_CREATE_NoOwner_Enforced:
// Unmanaged Pod (no ownerReferences) -> zero root timestamp must NOT skip enforcement.
// Without the !rootTimestamp.IsZero() guard, zero time is Before() any policy
// timestamp and would incorrectly skip a brand-new unmanaged resource.
func Test_allowExistingViolations_CREATE_NoOwner_Enforced(t *testing.T) {
	mockCL := func(_ context.Context, _ []kyvernov1.ContextEntry, _ enginecontext.Interface) error { return nil }
	client := &mockValidateClient{resources: map[string]*unstructured.Unstructured{}}

	policyContext := buildContext(t, kyvernov1.Create, allowExistingViolationsCreatePolicy, podUnmanagedNoTeamLabel, "")
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, client)

	resp := v.validate(context.TODO())
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusFail, resp.Status(),
		"unmanaged Pod with zero/no owner timestamp must not be skipped")
}

// Test_allowExistingViolations_CREATE_OwnerLookupFails_Enforced:
// Owner lookup returns error -> fail-safe: enforcement must proceed (Fail).
func Test_allowExistingViolations_CREATE_OwnerLookupFails_Enforced(t *testing.T) {
	mockCL := func(_ context.Context, _ []kyvernov1.ContextEntry, _ enginecontext.Interface) error { return nil }
	// empty store -> GetResource returns NotFound
	client := &mockValidateClient{resources: map[string]*unstructured.Unstructured{}}

	policyContext := buildContext(t, kyvernov1.Create, allowExistingViolationsCreatePolicy, podManagedNoTeamLabel, "")
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, client)

	resp := v.validate(context.TODO())
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusFail, resp.Status(),
		"owner lookup error must not skip enforcement (fail-safe)")
}

// Test_allowExistingViolations_UPDATE_OldObjectNotAffectedByCREATELogic:
// On UPDATE with both old+new violating, the result must be Skip via the
// Fail+Fail -> Skip UPDATE path. The CREATE owner-chain logic must NOT
// fire during validateOldObject() and turn priorResp into Skip.
func Test_allowExistingViolations_UPDATE_OldObjectNotAffectedByCREATELogic(t *testing.T) {
	mockCL := func(_ context.Context, _ []kyvernov1.ContextEntry, _ enginecontext.Interface) error { return nil }

	// Deployment predates policy - if CREATE logic leaked into validateOldObject
	// it would wrongly turn priorResp into Skip and break the UPDATE path.
	client := &mockValidateClient{resources: map[string]*unstructured.Unstructured{
		"ReplicaSet/default/test-rs":     makeRSObj("test-rs", "default", metav1.Date(2024, 1, 2, 0, 0, 0, 0, metav1.Now().Location()), "test-deploy"),
		"Deployment/default/test-deploy": makeDeployObj("test-deploy", "default", metav1.Date(2024, 1, 1, 0, 0, 0, 0, metav1.Now().Location())),
	}}

	// Both old and new violate -> Fail+Fail -> Skip via UPDATE allowExistingViolations
	policyContext := buildContext(t, kyvernov1.Update, allowExistingViolationsCreatePolicy, podManagedNoTeamLabel, podManagedNoTeamLabel)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule, client)

	resp := v.validate(context.TODO())
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusSkip, resp.Status(),
		"UPDATE Fail+Fail must Skip via UPDATE path; CREATE owner logic must not interfere with validateOldObject")
}
