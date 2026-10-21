package validation

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func Test_validateOldObject(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t, validateDenyPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

	ctx := context.TODO()
	resp := v.validate(ctx)
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusPass, resp.Status())

	rule2 := policyContext.Policy().GetSpec().Rules[1]
	v2 := newValidator(logr.Discard(), mockCL, policyContext, rule2)
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
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

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
	v := newValidator(logr.Discard(), failingCL, policyContext, rule)

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
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

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

type mockValidationClient struct {
	resources map[string]*unstructured.Unstructured
}

func (m *mockValidationClient) GetResource(_ context.Context, _ string, kind string, namespace string, name string, _ ...string) (*unstructured.Unstructured, error) {
	key := kind + "/" + namespace + "/" + name
	if obj, ok := m.resources[key]; ok {
		return obj, nil
	}
	return nil, fmt.Errorf("resource not found: %s", key)
}
func (m *mockValidationClient) ListResource(_ context.Context, _ string, _ string, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, nil
}
func (m *mockValidationClient) GetResources(_ context.Context, _, _, _, _, _, _ string, _ *metav1.LabelSelector) ([]api.Resource, error) {
	return nil, nil
}
func (m *mockValidationClient) GetNamespace(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Namespace, error) {
	return nil, nil
}
func (m *mockValidationClient) IsNamespaced(_, _, _ string) (bool, error) {
	return true, nil
}
func (m *mockValidationClient) CanI(_ context.Context, _, _, _, _, _ string) (bool, string, error) {
	return true, "", nil
}
func (m *mockValidationClient) RawAbsPath(_ context.Context, _ string, _ string, _ io.Reader) ([]byte, error) {
	return nil, nil
}
func (m *mockValidationClient) BuildRemoteOption(_ context.Context) ([]gcrremote.Option, error) {
	return nil, nil
}
func (m *mockValidationClient) BuildNameOption() []name.Option {
	return nil
}

func Test_AllowExistingViolations_ManagedResource_Create(t *testing.T) {
	engineutils.ResetOwnerCaches()

	rsTemplateHash := engineutils.ComputePodTemplateHash(&corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
	}, nil)

	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyJSON := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "require-team-label",
			"creationTimestamp": "2026-06-01T00:00:00Z"
		},
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-team",
				"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
				"validate": {
					"allowExistingViolations": true,
					"pattern": {
						"metadata": {
							"labels": {
								"team": "?*"
							}
						}
					}
				}
			}]
		}
	}`

	managedPodOldDeploy := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-old-deploy",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-old",
				"uid": "uid-rs-old",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	managedPodNewDeploy := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-new-deploy",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-new",
				"uid": "uid-rs-new",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	managedPodForgedUID := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-forged",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-old",
				"uid": "forged-fake-uid",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	managedPodNewRSUnderOldDeploy := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-new-rs-old-deploy",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-new-old-deploy",
				"uid": "uid-rs-new-old-deploy",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	managedPodMissingOwner := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-missing-owner",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-missing",
				"uid": "uid-rs-missing",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	managedPodOldRSTemplateUpdated := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-old-rs-template-updated",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-old-template-updated",
				"uid": "uid-rs-old-template-updated",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	unmanagedPod := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-standalone",
			"namespace": "default"
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`

	managedPodOldStatefulSetNoEvidence := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-sts-no-evidence",
			"namespace": "default",
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "StatefulSet",
				"name": "sts-old",
				"uid": "uid-sts-old",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`

	managedPodOldStatefulSetPreRevision := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-sts-pre-revision",
			"namespace": "default",
			"labels": {
				"app": "test",
				"controller-revision-hash": "prehash"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "StatefulSet",
				"name": "sts-old",
				"uid": "uid-sts-old",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`

	managedPodOldStatefulSetPostRevision := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-sts-post-revision",
			"namespace": "default",
			"labels": {
				"app": "test",
				"controller-revision-hash": "posthash"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "StatefulSet",
				"name": "sts-old",
				"uid": "uid-sts-old",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`

	managedPodOldDaemonSetNoEvidence := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-ds-no-evidence",
			"namespace": "default",
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "DaemonSet",
				"name": "ds-old",
				"uid": "uid-ds-old",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`

	managedPodOldDaemonSetPreRevision := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-ds-pre-revision",
			"namespace": "default",
			"labels": {
				"app": "test",
				"controller-revision-hash": "prehash"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "DaemonSet",
				"name": "ds-old",
				"uid": "uid-ds-old",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`

	managedPodStandaloneRS := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-standalone-rs",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-standalone",
				"uid": "uid-rs-standalone",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	makeObj := func(kind, namespace, name, uid string, ts metav1.Time, ownerKind, ownerName, ownerUID string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetKind(kind)
		obj.SetNamespace(namespace)
		obj.SetName(name)
		obj.SetUID(types.UID(uid))
		obj.SetCreationTimestamp(ts)
		if kind == "Job" || kind == "CronJob" {
			obj.SetAPIVersion("batch/v1")
		} else {
			obj.SetAPIVersion("apps/v1")
		}
		if ownerName != "" {
			controller := true
			obj.SetOwnerReferences([]metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       ownerKind,
				Name:       ownerName,
				UID:        types.UID(ownerUID),
				Controller: &controller,
			}})
		}
		if kind == "Deployment" || kind == "ReplicaSet" {
			_ = unstructured.SetNestedMap(obj.Object, map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "test"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx"},
					},
				},
			}, "spec", "template")
		}
		if kind == "ControllerRevision" {
			revTemplate := map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "test"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx"},
					},
				},
			}
			if ownerKind == "StatefulSet" {
				_ = unstructured.SetNestedMap(obj.Object, revTemplate, "data", "spec", "template")
			} else {
				_ = unstructured.SetNestedMap(obj.Object, revTemplate, "data", "template")
			}
		}
		if kind == "ReplicaSet" {
			obj.SetLabels(map[string]string{"pod-template-hash": rsTemplateHash})
			_ = unstructured.SetNestedField(obj.Object, rsTemplateHash, "spec", "selector", "matchLabels", "pod-template-hash")
		}
		return obj
	}

	deployOldTime := metav1.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rsOldTime := metav1.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	deployNewTime := metav1.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	rsNewTime := metav1.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC)

	rsOldTemplateUpdated := makeObj("ReplicaSet", "default", "rs-old-template-updated", "uid-rs-old-template-updated", rsOldTime, "Deployment", "deploy-old", "uid-deploy-old")
	_ = unstructured.SetNestedSlice(rsOldTemplateUpdated.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:tampered"},
	}, "spec", "template", "spec", "containers")
	rsOldTemplateUpdated.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "kubectl-edit",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		Time:       &rsNewTime,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{}}}`)},
	}})

	client := &mockValidationClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs-old":                   makeObj("ReplicaSet", "default", "rs-old", "uid-rs-old", rsOldTime, "Deployment", "deploy-old", "uid-deploy-old"),
			"Deployment/default/deploy-old":               makeObj("Deployment", "default", "deploy-old", "uid-deploy-old", deployOldTime, "", "", ""),
			"ReplicaSet/default/rs-new":                   makeObj("ReplicaSet", "default", "rs-new", "uid-rs-new", rsNewTime, "Deployment", "deploy-new", "uid-deploy-new"),
			"Deployment/default/deploy-new":               makeObj("Deployment", "default", "deploy-new", "uid-deploy-new", deployNewTime, "", "", ""),
			"ReplicaSet/default/rs-new-old-deploy":        makeObj("ReplicaSet", "default", "rs-new-old-deploy", "uid-rs-new-old-deploy", rsNewTime, "Deployment", "deploy-old", "uid-deploy-old"),
			"ReplicaSet/default/rs-old-template-updated":  rsOldTemplateUpdated,
			"StatefulSet/default/sts-old":                 makeObj("StatefulSet", "default", "sts-old", "uid-sts-old", rsOldTime, "", "", ""),
			"DaemonSet/default/ds-old":                    makeObj("DaemonSet", "default", "ds-old", "uid-ds-old", rsOldTime, "", "", ""),
			"ControllerRevision/default/sts-old-prehash":  makeObj("ControllerRevision", "default", "sts-old-prehash", "uid-sts-rev-pre", rsOldTime, "StatefulSet", "sts-old", "uid-sts-old"),
			"ControllerRevision/default/sts-old-posthash": makeObj("ControllerRevision", "default", "sts-old-posthash", "uid-sts-rev-post", rsNewTime, "StatefulSet", "sts-old", "uid-sts-old"),
			"ControllerRevision/default/ds-old-prehash":   makeObj("ControllerRevision", "default", "ds-old-prehash", "uid-ds-rev-pre", rsOldTime, "DaemonSet", "ds-old", "uid-ds-old"),
			"ReplicaSet/default/rs-standalone":            makeObj("ReplicaSet", "default", "rs-standalone", "uid-rs-standalone", rsOldTime, "", "", ""),
		},
	}

	tests := []struct {
		name        string
		operation   kyvernov1.AdmissionOperation
		isAdmission bool
		resource    string
		oldResource string
		userInfo    *authenticationv1.UserInfo
		wantStatus  api.RuleStatus
	}{
		{
			name:        "managed pod whose root owner predates policy is skipped on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldDeploy,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusSkip,
		},
		{
			name:        "managed pod with empty username fails on CREATE (unauthenticated request)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldDeploy,
			userInfo: &authenticationv1.UserInfo{
				Username: "",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod with untrusted user identity fails on CREATE (forgery prevention)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldDeploy,
			userInfo: &authenticationv1.UserInfo{
				Username: "untrusted-user",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod with non-controller service account fails on CREATE (allowlist restriction)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldDeploy,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:ci",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod under new ReplicaSet of old Deployment fails on CREATE (template update)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodNewRSUnderOldDeploy,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod in background scan (non-admission) fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: false,
			resource:    managedPodOldDeploy,
			wantStatus:  api.RuleStatusFail,
		},
		{
			name:        "managed pod whose root owner was created after policy fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodNewDeploy,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "forged owner reference with mismatched UID fails validation on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodForgedUID,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "owner lookup error (missing owner) fails validation on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodMissingOwner,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "unmanaged standalone pod without owners fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    unmanagedPod,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod under old StatefulSet without template-revision evidence fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldStatefulSetNoEvidence,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod under old StatefulSet with pre-policy revision is skipped on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldStatefulSetPreRevision,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			wantStatus: api.RuleStatusSkip,
		},
		{
			name:        "managed pod under old StatefulSet with post-policy revision fails on CREATE (template update)",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldStatefulSetPostRevision,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:statefulset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod under old DaemonSet without template-revision evidence fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldDaemonSetNoEvidence,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:daemon-set-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod under old DaemonSet with pre-policy revision is skipped on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldDaemonSetPreRevision,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:daemon-set-controller",
			},
			wantStatus: api.RuleStatusSkip,
		},
		{
			name:        "managed pod under standalone ReplicaSet fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodStandaloneRS,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "managed pod under pre-policy ReplicaSet with post-policy template update fails on CREATE",
			operation:   kyvernov1.Create,
			isAdmission: true,
			resource:    managedPodOldRSTemplateUpdated,
			userInfo: &authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:replicaset-controller",
			},
			wantStatus: api.RuleStatusFail,
		},
		{
			name:        "UPDATE request with existing violation continues to skip via update path",
			operation:   kyvernov1.Update,
			isAdmission: true,
			resource:    managedPodOldDeploy,
			oldResource: managedPodOldDeploy,
			wantStatus:  api.RuleStatusSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineutils.ResetOwnerCaches()
			rawCtx := buildContext(t, tt.operation, policyJSON, tt.resource, tt.oldResource)
			policyCtx := rawCtx
			if tt.isAdmission {
				if pc, ok := rawCtx.(*policycontext.PolicyContext); ok {
					pc = pc.WithAdmissionOperation(true)
					if tt.userInfo != nil {
						pc = pc.WithAdmissionInfo(kyvernov2.RequestInfo{
							AdmissionUserInfo: *tt.userInfo,
						})
					}
					policyCtx = pc
				}
			}
			rule := policyCtx.Policy().GetSpec().Rules[0]
			v := newValidator(logr.Discard(), mockCL, policyCtx, rule).withClient(client)

			resp := v.validate(context.TODO())
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantStatus, resp.Status())
		})
	}
}

func Test_AllowExistingViolations_LiveDirectEvaluation_MutatedTemplateWithoutCacheReset_Fails(t *testing.T) {
	policyJSON := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "require-team-label",
			"creationTimestamp": "2026-06-01T00:00:00Z"
		},
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-team",
				"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
				"validate": {
					"allowExistingViolations": true,
					"pattern": {
						"metadata": {
							"labels": {
								"team": "?*"
							}
						}
					}
				}
			}]
		}
	}`

	rsTemplateHash := engineutils.ComputePodTemplateHash(&corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
	}, nil)

	managedPod := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "pod-live-test",
			"namespace": "default",
			"labels": {
				"app": "test",
				"pod-template-hash": "%s"
			},
			"ownerReferences": [{
				"apiVersion": "apps/v1",
				"kind": "ReplicaSet",
				"name": "rs-live",
				"uid": "uid-rs-live",
				"controller": true
			}]
		},
		"spec": {
			"containers": [{"name": "nginx", "image": "nginx"}]
		}
	}`, rsTemplateHash)

	deployOldTime := metav1.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	rsOldTime := metav1.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	makeObj := func(kind, namespace, name, uid string, ts metav1.Time, ownerKind, ownerName, ownerUID string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetKind(kind)
		obj.SetNamespace(namespace)
		obj.SetName(name)
		obj.SetUID(types.UID(uid))
		obj.SetCreationTimestamp(ts)
		obj.SetAPIVersion("apps/v1")
		if ownerKind != "" {
			obj.SetOwnerReferences([]metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       ownerKind,
				Name:       ownerName,
				UID:        types.UID(ownerUID),
				Controller: ptr.To(true),
			}})
		}
		if kind == "Deployment" || kind == "ReplicaSet" {
			_ = unstructured.SetNestedMap(obj.Object, map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "test"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "nginx", "image": "nginx"},
					},
				},
			}, "spec", "template")
		}
		if kind == "ReplicaSet" {
			obj.SetLabels(map[string]string{"pod-template-hash": rsTemplateHash})
			_ = unstructured.SetNestedField(obj.Object, rsTemplateHash, "spec", "selector", "matchLabels", "pod-template-hash")
		}
		return obj
	}

	client := &mockValidationClient{
		resources: map[string]*unstructured.Unstructured{
			"ReplicaSet/default/rs-live":     makeObj("ReplicaSet", "default", "rs-live", "uid-rs-live", rsOldTime, "Deployment", "deploy-live", "uid-deploy-live"),
			"Deployment/default/deploy-live": makeObj("Deployment", "default", "deploy-live", "uid-deploy-live", deployOldTime, "", "", ""),
		},
	}

	runValidation := func() api.RuleStatus {
		rawCtx := buildContext(t, kyvernov1.Create, policyJSON, managedPod, "")
		pc := rawCtx.(*policycontext.PolicyContext).
			WithAdmissionOperation(true).
			WithAdmissionInfo(kyvernov2.RequestInfo{
				AdmissionUserInfo: authenticationv1.UserInfo{
					Username: "system:serviceaccount:kube-system:replicaset-controller",
				},
			})
		rule := pc.Policy().GetSpec().Rules[0]
		v := newValidator(logr.Discard(), func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
			return nil
		}, pc, rule).withClient(client)
		resp := v.validate(context.TODO())
		assert.NotNil(t, resp)
		return resp.Status()
	}

	// 1. First evaluation: pre-existing owner allows creation (RuleStatusSkip)
	status1 := runValidation()
	assert.Equal(t, api.RuleStatusSkip, status1)

	// 2. Mutate ReplicaSet template in mock client WITHOUT resetting cache
	rsMutated := makeObj("ReplicaSet", "default", "rs-live", "uid-rs-live", rsOldTime, "Deployment", "deploy-live", "uid-deploy-live")
	_ = unstructured.SetNestedSlice(rsMutated.Object, []any{
		map[string]any{"name": "nginx", "image": "nginx:tampered"},
	}, "spec", "template", "spec", "containers")
	client.resources["ReplicaSet/default/rs-live"] = rsMutated

	// 3. Second evaluation: reads live mutated state directly and fails (RuleStatusFail)
	status2 := runValidation()
	assert.Equal(t, api.RuleStatusFail, status2)
}
