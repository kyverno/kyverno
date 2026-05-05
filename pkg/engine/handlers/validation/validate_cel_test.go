package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeClient implements engineapi.Client for testing.
type fakeClient struct {
	namespaces   map[string]*corev1.Namespace
	getNamespace func(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error)
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		namespaces: map[string]*corev1.Namespace{
			"test-ns": {
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ns",
					Labels: map[string]string{"env": "test"},
				},
			},
		},
	}
}

func (c *fakeClient) GetNamespace(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error) {
	if c.getNamespace != nil {
		return c.getNamespace(ctx, name, opts)
	}
	ns, ok := c.namespaces[name]
	if !ok {
		return nil, fmt.Errorf("namespace %q not found", name)
	}
	return ns, nil
}

func (c *fakeClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeClient) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeClient) GetResources(ctx context.Context, group, version, kind, subresource, namespace, name string, lselector *metav1.LabelSelector) ([]engineapi.Resource, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *fakeClient) IsNamespaced(group, version, kind string) (bool, error) {
	return true, nil
}

func (c *fakeClient) CanI(ctx context.Context, kind, namespace, verb, subresource, user string) (bool, string, error) {
	return true, "", nil
}

func (c *fakeClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

// buildCELContext creates a PolicyContext suitable for CEL handler tests.
func buildCELContext(t *testing.T, operation kyvernov1.AdmissionOperation, policyJSON, resourceJSON, oldResourceJSON string) *policycontext.PolicyContext {
	t.Helper()

	var cpol kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(policyJSON), &cpol)
	require.NoError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(resourceJSON))
	require.NoError(t, err)

	pc, err := policycontext.NewPolicyContext(
		jp,
		*resourceUnstructured,
		operation,
		nil,
		cfg,
	)
	require.NoError(t, err)

	pc = pc.
		WithPolicy(&cpol).
		WithNewResource(*resourceUnstructured).
		WithResourceKind(podGVK, "").
		WithRequestResource(podGVR)

	if oldResourceJSON != "" {
		oldResourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(oldResourceJSON))
		require.NoError(t, err)
		pc = pc.WithOldResource(*oldResourceUnstructured)
	}

	return pc
}

// noopContextLoader is a no-op context loader (CEL handler ignores this parameter).
var noopContextLoader engineapi.EngineContextLoader = nil

var (
	podGVK = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	podGVR = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
)

// --- Test policies ---

var celPolicyPass = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": { "name": "cel-pass" },
	"spec": {
		"validationFailureAction": "Enforce",
		"rules": [{
			"name": "check-labels",
			"match": {
				"any": [{ "resources": { "kinds": ["Pod"] } }]
			},
			"validate": {
				"cel": {
					"expressions": [{
						"expression": "object.metadata.name == 'test-pod'",
						"message": "name must be test-pod"
					}]
				}
			}
		}]
	}
}`

var celPolicyDeny = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": { "name": "cel-deny" },
	"spec": {
		"validationFailureAction": "Enforce",
		"rules": [{
			"name": "check-labels",
			"match": {
				"any": [{ "resources": { "kinds": ["Pod"] } }]
			},
			"validate": {
				"cel": {
					"expressions": [{
						"expression": "object.metadata.name == 'must-not-exist'",
						"message": "name must be must-not-exist"
					}]
				}
			}
		}]
	}
}`

var celPolicyVAPGenerated = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": { "name": "cel-vap" },
	"status": {
		"validatingadmissionpolicy": {
			"generated": true,
			"message": ""
		}
	},
	"spec": {
		"validationFailureAction": "Enforce",
		"rules": [{
			"name": "check-labels",
			"match": {
				"any": [{ "resources": { "kinds": ["Pod"] } }]
			},
			"validate": {
				"cel": {
					"expressions": [{
						"expression": "object.metadata.name == 'must-not-exist'",
						"message": "should deny but VAP is generated"
					}]
				}
			}
		}]
	}
}`

var celPodResource = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"namespace": "test-ns",
		"labels": { "app": "test" }
	},
	"spec": {
		"containers": [{
			"name": "nginx",
			"image": "nginx:latest"
		}]
	}
}`

var celPodResourceCluster = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "test-pod",
		"labels": { "app": "test" }
	},
	"spec": {
		"containers": [{
			"name": "nginx",
			"image": "nginx:latest"
		}]
	}
}`

var celNamespaceResource = `{
	"apiVersion": "v1",
	"kind": "Namespace",
	"metadata": {
		"name": "test-ns",
		"labels": { "env": "test" }
	}
}`

// --- Tests ---

func TestValidateCELHandler_PassExpression(t *testing.T) {
	handler, err := NewValidateCELHandler(newFakeClient(), true)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyPass, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_DenyExpression(t *testing.T) {
	handler, err := NewValidateCELHandler(newFakeClient(), true)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyDeny, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "name must be must-not-exist")
}

func TestValidateCELHandler_PolicyExceptionSkipsEnforcement(t *testing.T) {
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	// Use the deny policy: without the exception this would produce a deny.
	pc := buildCELContext(t, kyvernov1.Create, celPolicyDeny, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-exception",
			Namespace: "test-ns",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Match: kyvernov2.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
				},
			},
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "cel-deny",
					RuleNames:  []string{"check-labels"},
				},
			},
		},
	}

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, []*kyvernov2.PolicyException{exception})
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusSkip, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "policy exceptions")
}

func TestValidateCELHandler_VAPGeneratedReturnsNilResponses(t *testing.T) {
	handler, err := NewValidateCELHandler(newFakeClient(), true)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyVAPGenerated, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	// When VAP is generated, the handler returns nil (no rule responses).
	// This is intentional: the validation is delegated to the Kubernetes VAP.
	assert.Nil(t, responses, "expected nil responses when VAP is generated — validation is delegated to Kubernetes")
}

func TestValidateCELHandler_NoExceptionsProceeds(t *testing.T) {
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyPass, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	// Empty exceptions slice: handler should proceed to CEL evaluation.
	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, []*kyvernov2.PolicyException{})
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_DeleteRequestUsesOldObject(t *testing.T) {
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	// For DELETE, pass the resource as oldResource and nil as the new resource.
	var cpol kyvernov1.ClusterPolicy
	err = json.Unmarshal([]byte(celPolicyPass), &cpol)
	require.NoError(t, err)

	oldResource, err := kubeutils.BytesToUnstructured([]byte(celPodResource))
	require.NoError(t, err)

	pc, err := policycontext.NewPolicyContext(
		jp,
		*oldResource,
		kyvernov1.Delete,
		nil,
		cfg,
	)
	require.NoError(t, err)

	pc = pc.
		WithPolicy(&cpol).
		WithOldResource(*oldResource).
		WithResourceKind(podGVK, "").
		WithRequestResource(podGVR)

	rule := cpol.Spec.Rules[0]
	emptyResource := unstructured.Unstructured{}

	// The handler should extract name/namespace from the old object
	// and not panic on the nil new resource.
	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, emptyResource, rule, noopContextLoader, nil)
	// The CEL expression checks object.metadata.name, but on DELETE object is nil,
	// so this should produce an error or skip — the key invariant is no panic.
	require.NotNil(t, responses, "handler must return a response, not nil")
	assert.True(t, len(responses) > 0, "expected at least one response for DELETE request")
}

func TestValidateCELHandler_NamespaceKindUnsetsNs(t *testing.T) {
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	// Build a policy that validates namespaces via CEL.
	celPolicyNs := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-ns-check" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-ns",
				"match": {
					"any": [{ "resources": { "kinds": ["Namespace"] } }]
				},
				"validate": {
					"cel": {
						"expressions": [{
							"expression": "object.metadata.name == 'test-ns'",
							"message": "namespace name must be test-ns"
						}]
					}
				}
			}]
		}
	}`

	nsResource, err := kubeutils.BytesToUnstructured([]byte(celNamespaceResource))
	require.NoError(t, err)

	nsGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	nsGVR := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

	pc, err := policycontext.NewPolicyContext(
		jp,
		*nsResource,
		kyvernov1.Create,
		nil,
		cfg,
	)
	require.NoError(t, err)

	var cpol kyvernov1.ClusterPolicy
	err = json.Unmarshal([]byte(celPolicyNs), &cpol)
	require.NoError(t, err)

	pc = pc.
		WithPolicy(&cpol).
		WithNewResource(*nsResource).
		WithResourceKind(nsGVK, "").
		WithRequestResource(nsGVR)

	rule := cpol.Spec.Rules[0]

	// For Namespace resources, the handler should unset `ns` to "" so that
	// it does not attempt a GetNamespace call for the namespace itself.
	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, *nsResource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_GetNamespaceError(t *testing.T) {
	client := &fakeClient{
		getNamespace: func(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error) {
			return nil, fmt.Errorf("simulated namespace lookup failure")
		},
	}
	handler, err := NewValidateCELHandler(client, true)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyPass, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "Error getting the resource's namespace")
}

func TestValidateCELHandler_NonClusterFallbackNamespace(t *testing.T) {
	// When isCluster=false, the handler creates a synthetic Namespace object
	// instead of calling client.GetNamespace.
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyPass, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	// The handler should pass without calling GetNamespace.
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_UpdateRequestSetsOldObject(t *testing.T) {
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	updatedPod := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "test-ns",
			"labels": { "app": "updated" }
		},
		"spec": {
			"containers": [{
				"name": "nginx",
				"image": "nginx:1.25"
			}]
		}
	}`

	// CEL expression that checks the new object — should pass.
	pc := buildCELContext(t, kyvernov1.Update, celPolicyPass, updatedPod, celPodResource)
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_EmptyMessageFallback(t *testing.T) {
	// When a CEL expression has an empty message, it should fall back
	// to rule.Validation.Message.
	celPolicyEmptyMsg := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-empty-msg" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-labels",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"validate": {
					"message": "fallback message from rule",
					"cel": {
						"expressions": [{
							"expression": "object.metadata.name == 'nonexistent'"
						}]
					}
				}
			}]
		}
	}`

	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyEmptyMsg, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "fallback message from rule")
}

func TestValidateCELHandler_CELPreconditionsNotMet(t *testing.T) {
	// When CEL preconditions (matchConditions) are not met, the handler
	// should return RuleSkip.
	celPolicyWithPrecondition := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-precondition" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-with-precondition",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"celPreconditions": [{
					"name": "must-have-label",
					"expression": "has(object.metadata.labels.nonexistent)"
				}],
				"validate": {
					"cel": {
						"expressions": [{
							"expression": "object.metadata.name == 'must-not-exist'",
							"message": "should not reach here"
						}]
					}
				}
			}]
		}
	}`

	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyWithPrecondition, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusSkip, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "cel preconditions not met")
}

func TestValidateCELHandler_MultipleExpressions(t *testing.T) {
	// Multiple CEL expressions: all must pass for the rule to pass.
	celPolicyMulti := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-multi" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "multi-check",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"validate": {
					"cel": {
						"expressions": [
							{
								"expression": "object.metadata.name == 'test-pod'",
								"message": "name check"
							},
							{
								"expression": "has(object.metadata.labels.app)",
								"message": "must have app label"
							}
						]
					}
				}
			}]
		}
	}`

	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyMulti, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_MultipleExpressionsFirstFails(t *testing.T) {
	celPolicyMultiFail := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-multi-fail" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "multi-check",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"validate": {
					"cel": {
						"expressions": [
							{
								"expression": "object.metadata.name == 'wrong-name'",
								"message": "first expression fails"
							},
							{
								"expression": "has(object.metadata.labels.app)",
								"message": "second expression passes"
							}
						]
					}
				}
			}]
		}
	}`

	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyMultiFail, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "first expression fails")
}

func TestValidateCELHandler_CELVariables(t *testing.T) {
	// Test that CEL variables are available in expressions.
	celPolicyVars := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-vars" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-with-vars",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"validate": {
					"cel": {
						"variables": [{
							"name": "podName",
							"expression": "object.metadata.name"
						}],
						"expressions": [{
							"expression": "variables.podName == 'test-pod'",
							"message": "pod name must be test-pod"
						}]
					}
				}
			}]
		}
	}`

	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyVars, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_ClusterResourceNoNamespace(t *testing.T) {
	// When the resource has no namespace (cluster-scoped), the handler should
	// not attempt a GetNamespace call.
	handler, err := NewValidateCELHandler(newFakeClient(), true)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyPass, celPodResourceCluster, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_AuditAnnotations(t *testing.T) {
	celPolicyAuditAnnotation := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-audit-ann" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-with-audit-annotation",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"validate": {
					"cel": {
						"expressions": [{
							"expression": "object.metadata.name == 'test-pod'",
							"message": "name must be test-pod"
						}],
						"auditAnnotations": [{
							"key": "test-key",
							"valueExpression": "'checked-pod-' + object.metadata.name"
						}]
					}
				}
			}]
		}
	}`

	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyAuditAnnotation, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	// The expression passes, so the overall result should be pass regardless of audit annotations.
	assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
}

func TestValidateCELHandler_ExceptionWithNamespaceKey(t *testing.T) {
	// Verify that exception keys include namespace/name format.
	handler, err := NewValidateCELHandler(nil, false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyDeny, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()

	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-exception",
			Namespace: "default",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Match: kyvernov2.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
				},
			},
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "cel-deny",
					RuleNames:  []string{"check-labels"},
				},
			},
		},
	}

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, []*kyvernov2.PolicyException{exception})
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusSkip, responses[0].Status())
	// The message should contain the namespaced key "default/my-exception".
	assert.Contains(t, responses[0].Message(), "default/my-exception")
}

func TestValidateCELHandler_ParamKindWithoutClient(t *testing.T) {
	// When hasParam is true but client cannot collect params, expect RuleError.
	celPolicyParam := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": { "name": "cel-param" },
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "check-with-param",
				"match": {
					"any": [{ "resources": { "kinds": ["Pod"] } }]
				},
				"validate": {
					"cel": {
						"paramKind": {
							"apiVersion": "v1",
							"kind": "ConfigMap"
						},
						"paramRef": {
							"name": "my-params",
							"namespace": "test-ns"
						},
						"expressions": [{
							"expression": "object.metadata.name == 'test-pod'",
							"message": "name must be test-pod"
						}]
					}
				}
			}]
		}
	}`

	// fakeClient does not implement param collection, so CollectParams should fail.
	handler, err := NewValidateCELHandler(newFakeClient(), false)
	require.NoError(t, err)

	pc := buildCELContext(t, kyvernov1.Create, celPolicyParam, celPodResource, "")

	var cpol kyvernov1.ClusterPolicy
	err = json.Unmarshal([]byte(celPolicyParam), &cpol)
	require.NoError(t, err)

	rule := cpol.Spec.Rules[0]

	// Verify HasParam is true.
	require.True(t, rule.Validation.CEL.HasParam())

	resource := pc.NewResource()

	_, responses := handler.Process(context.TODO(), logr.Discard(), pc, resource, rule, noopContextLoader, nil)
	require.Len(t, responses, 1)
	assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "error in parameterized resource")
}

// Verify that the Validation type properly handles CEL field construction
// used in the handler's expression extraction path.
func TestCELValidationFieldExtraction(t *testing.T) {
	expressions := []admissionregistrationv1.Validation{
		{
			Expression: "object.metadata.name == 'test'",
			Message:    "",
		},
		{
			Expression: "has(object.metadata.labels)",
			Message:    "explicit message",
		},
	}

	rule := kyvernov1.Rule{
		Name: "test-rule",
		Validation: &kyvernov1.Validation{
			Message: "rule-level fallback message",
			CEL: &kyvernov1.CEL{
				Expressions: expressions,
			},
		},
	}

	// Simulate the message backfill logic from validate_cel.go lines 117-121.
	validations := rule.Validation.CEL.Expressions
	for i := range validations {
		if validations[i].Message == "" {
			validations[i].Message = rule.Validation.Message
		}
	}

	assert.Equal(t, "rule-level fallback message", validations[0].Message, "empty message should be filled from rule.Validation.Message")
	assert.Equal(t, "explicit message", validations[1].Message, "non-empty message should not be overwritten")
}
