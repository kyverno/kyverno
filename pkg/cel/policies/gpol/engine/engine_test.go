package engine

import (
	"context"
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ctx1 = libs.NewFakeContextProvider()

	matcher    = matching.NewMatcher()
	nsResolver = func(ns string) *corev1.Namespace {
		return nil
	}

	eng = NewEngine(nsResolver, matcher)

	resource = unstructured.Unstructured{}
	obj      = unstructured.Unstructured{}
	oldObj   = unstructured.Unstructured{}

	gvk = schema.GroupVersionKind{
		Group:   "",
		Version: "",
		Kind:    "",
	}
	gvr = schema.GroupVersionResource{
		Group:    "",
		Version:  "",
		Resource: "",
	}
	req = engine.Request(
		ctx1,
		gvk,
		gvr,
		"",
		resource.GetName(),
		resource.GetNamespace(),
		admissionv1.Connect,
		v1.UserInfo{},
		&obj,
		&oldObj,
		false,
		nil,
	)
)

type recordingContext struct {
	libs.Context
	triggerUID       string
	triggerNamespace string
}

func (r *recordingContext) SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache, useServerSideApply bool) {
	r.triggerUID = triggerUID
	r.triggerNamespace = triggerNamespace
	r.Context.SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID, restoreCache, useServerSideApply)
}

func (r *recordingContext) Clone() libs.Context {
	return &recordingContext{
		Context:          r.Context.Clone(),
		triggerUID:       r.triggerUID,
		triggerNamespace: r.triggerNamespace,
	}
}

func TestHandle(t *testing.T) {
	t.Run("should handle policy with match constraints and return response", func(t *testing.T) {
		pol := Policy{
			Policy: &v1beta1.GeneratingPolicy{
				Spec: v1beta1.GeneratingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								ResourceNames: []string{"pods"},
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"},
										Resources:   []string{"pods"},
									},
								},
							},
						},
					},
				},
			},
		}

		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should return error when ExtractResources fails due to empty request", func(t *testing.T) {
		req := engine.EngineRequest{
			Request: admissionv1.AdmissionRequest{},
		}

		resp, err := eng.Handle(context.TODO(), req, Policy{}, false)
		assert.Nil(t, err)
		assert.NotNil(t, resp.Policies)
	})

	t.Run("should handle policy with isolated namespace and match constraints", func(t *testing.T) {
		resource.SetNamespace("isolated")
		req := engine.Request(
			ctx1,
			gvk,
			gvr,
			"",
			resource.GetName(),
			resource.GetNamespace(),
			admissionv1.Connect,
			v1.UserInfo{},
			&obj,
			&oldObj,
			false,
			nil,
		)
		pol := Policy{
			Policy: &v1beta1.GeneratingPolicy{
				Spec: v1beta1.GeneratingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								ResourceNames: []string{"pods"},
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"},
										Resources:   []string{"pods"},
									},
								},
							},
						},
					},
				},
			},
		}

		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate policy with valid match condition on default namespace", func(t *testing.T) {
		pol := Policy{
			Policy: &v1beta1.GeneratingPolicy{
				Spec: v1beta1.GeneratingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "valid-namespace",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
		}

		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate policy with match condition but different namespace", func(t *testing.T) {
		resource.SetName("valid")
		resource.SetNamespace("valid-ns")

		pol := Policy{
			Policy: &v1beta1.GeneratingPolicy{
				Spec: v1beta1.GeneratingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "valid-namespace",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
		}

		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate compiled policy without exceptions", func(t *testing.T) {
		gpol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "valid-namespace",
						Expression: "object.metadata.namespace == 'default'",
					},
				},
			},
		}
		comp := compiler.NewCompiler()
		compiledGpol, _ := comp.Compile(gpol, nil)

		pol := Policy{
			Exceptions:     nil,
			Policy:         gpol,
			CompiledPolicy: compiledGpol,
		}
		eng := NewEngine(nsResolver, nil)
		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("NamespacedGeneratingPolicy rejects namespace arg at compile time", func(t *testing.T) {
		ngpol := &v1beta1.NamespacedGeneratingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cross-ns-escalate",
				Namespace: "tenant-ns",
			},
			Spec: v1beta1.GeneratingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"configmaps"},
								},
							},
						},
					},
				},
				Generation: []v1beta1.Generation{
					{
						Expression: `generator.Apply("kube-system", [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap")}])`,
					},
				},
			},
		}
		comp := compiler.NewCompiler()
		_, errList := comp.Compile(ngpol, nil)
		assert.NotNil(t, errList, "namespace arg must be rejected at compile time for namespaced policies")
	})

	t.Run("should evaluate compiled policy with variable expressions and policy exceptions", func(t *testing.T) {
		obj.SetNamespace("default")
		gpol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				Variables: []admissionregistrationv1.Variable{
					{
						Name:       "apiResponse",
						Expression: "http.Get('http://test-api-service.default.svc.cluster.local:80')",
					},
					{
						Name:       "envLabel",
						Expression: "has(variables.apiResponse) && has(variables.apiResponse.metadata) && has(variables.apiResponse.metadata.labels) && 'app' in variables.apiResponse.metadata.labels ? variables.apiResponse.metadata.labels.app : 'unknown'",
					},
					{
						Name:       "nsName",
						Expression: "object.metadata.name",
					},
				},
				Generation: []v1beta1.Generation{
					{
						Expression: "generator.Apply(variables.nsName, variables.nsName)",
					},
				},
			},
		}
		exceptions := []*v1beta1.PolicyException{
			{
				Spec: v1beta1.PolicyExceptionSpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "valid-namespace",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
		}
		comp := compiler.NewCompiler()
		compiledGpol, _ := comp.Compile(gpol, exceptions)

		pol := Policy{
			Exceptions:     exceptions,
			Policy:         gpol,
			CompiledPolicy: compiledGpol,
		}
		eng := NewEngine(nsResolver, nil)
		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should surface audit annotations as rule response properties", func(t *testing.T) {
		gpol := &v1beta1.GeneratingPolicy{
			Spec: v1beta1.GeneratingPolicySpec{
				AuditAnnotations: []admissionregistrationv1.AuditAnnotation{
					{
						Key:             "triggeredBy",
						ValueExpression: "'ns/' + object.metadata.namespace",
					},
				},
			},
		}
		comp := compiler.NewCompiler()
		compiledGpol, errs := comp.Compile(gpol, nil)
		assert.Nil(t, errs)

		pol := Policy{
			Policy:         gpol,
			CompiledPolicy: compiledGpol,
		}
		eng := NewEngine(nsResolver, nil)
		resp, err := eng.Handle(context.TODO(), req, pol, false)
		assert.NoError(t, err)
		assert.Len(t, resp.Policies, 1)
		result := resp.Policies[0].Result
		assert.NotNil(t, result)
		// Audit annotations are always surfaced on successful evaluation
		assert.Equal(t, "ns/default", result.Properties()["triggeredBy"])
	})

	t.Run("should match namespace and pass trigger UID on trigger DELETE operation", func(t *testing.T) {
		deletedObj := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "test-cm",
					"namespace": "prod-apps",
					"uid":       "cm-uid-12345",
				},
			},
		}

		recCtx := &recordingContext{Context: libs.NewFakeContextProvider()}
		deleteReq := engine.Request(
			recCtx,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			"",
			"test-cm",
			"prod-apps",
			admissionv1.Delete,
			v1.UserInfo{},
			nil,
			&deletedObj,
			false,
			nil,
		)

		gpol := &v1beta1.GeneratingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "generate-on-delete",
			},
			Spec: v1beta1.GeneratingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"environment": "prod"},
					},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"configmaps"},
								},
							},
						},
					},
				},
				Generation: []v1beta1.Generation{
					{
						Expression: `generator.Apply("prod-apps", [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": {"name": dyn("backup-cm")}}])`,
					},
				},
			},
		}

		comp := compiler.NewCompiler()
		compiledGpol, errs := comp.Compile(gpol, nil)
		assert.Empty(t, errs)

		pol := Policy{
			Policy:         gpol,
			CompiledPolicy: compiledGpol,
		}

		var resolvedNamespace string
		engWithMatcher := NewEngine(func(ns string) *corev1.Namespace {
			resolvedNamespace = ns
			return &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   ns,
					Labels: map[string]string{"environment": "prod"},
				},
			}
		}, matcher)

		resp, err := engWithMatcher.Handle(context.TODO(), deleteReq, pol, false)
		assert.NoError(t, err)
		assert.Equal(t, "prod-apps", resolvedNamespace)
		assert.Equal(t, "cm-uid-12345", recCtx.triggerUID)
		assert.Equal(t, "prod-apps", recCtx.triggerNamespace)
		assert.NotNil(t, resp)
		assert.Equal(t, &deletedObj, resp.Trigger)
		assert.Len(t, resp.Policies, 1)
		if assert.NotNil(t, resp.Policies[0].Result) {
			assert.Equal(t, engineapi.RuleStatusPass, resp.Policies[0].Result.Status())
		}
	})

	t.Run("should fallback to trigger namespace when request namespace is empty on DELETE", func(t *testing.T) {
		gpol := &v1beta1.GeneratingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "generate-fallback-ns",
			},
			Spec: v1beta1.GeneratingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "fallback-ns"},
					},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"configmaps"},
								},
							},
						},
					},
				},
				Generation: []v1beta1.Generation{
					{
						Expression: `generator.Apply("fallback-ns", [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": {"name": dyn("cm-out")}}])`,
					},
				},
			},
		}

		comp := compiler.NewCompiler()
		compiledGpol, errs := comp.Compile(gpol, nil)
		assert.Empty(t, errs)

		pol := Policy{
			Policy:         gpol,
			CompiledPolicy: compiledGpol,
		}

		t.Run("matching fallback namespace", func(t *testing.T) {
			deletedObj := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-cm",
						"namespace": "fallback-ns",
						"uid":       "cm-uid-67890",
					},
				},
			}

			recCtx := &recordingContext{Context: libs.NewFakeContextProvider()}
			deleteReq := engine.Request(
				recCtx,
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
				schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
				"",
				"test-cm",
				"", // empty request namespace
				admissionv1.Delete,
				v1.UserInfo{},
				nil,
				&deletedObj,
				false,
				nil,
			)

			var resolvedNamespace string
			engWithMatcher := NewEngine(func(ns string) *corev1.Namespace {
				resolvedNamespace = ns
				return &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   ns,
						Labels: map[string]string{"kubernetes.io/metadata.name": ns},
					},
				}
			}, matcher)

			resp, err := engWithMatcher.Handle(context.TODO(), deleteReq, pol, false)
			assert.NoError(t, err)
			assert.Equal(t, "fallback-ns", resolvedNamespace)
			assert.Equal(t, "cm-uid-67890", recCtx.triggerUID)
			assert.Equal(t, "fallback-ns", recCtx.triggerNamespace)
			assert.NotNil(t, resp)
			assert.Equal(t, &deletedObj, resp.Trigger)
			assert.Len(t, resp.Policies, 1)
			if assert.NotNil(t, resp.Policies[0].Result) {
				assert.Equal(t, engineapi.RuleStatusPass, resp.Policies[0].Result.Status())
			}
		})

		t.Run("non-matching fallback namespace is skipped", func(t *testing.T) {
			deletedObj := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-cm",
						"namespace": "other-ns",
						"uid":       "cm-uid-67890",
					},
				},
			}

			deleteReq := engine.Request(
				ctx1,
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
				schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
				"",
				"test-cm",
				"", // empty request namespace
				admissionv1.Delete,
				v1.UserInfo{},
				nil,
				&deletedObj,
				false,
				nil,
			)

			var resolvedNamespace string
			engWithMatcher := NewEngine(func(ns string) *corev1.Namespace {
				resolvedNamespace = ns
				return &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   ns,
						Labels: map[string]string{"kubernetes.io/metadata.name": ns},
					},
				}
			}, matcher)

			resp, err := engWithMatcher.Handle(context.TODO(), deleteReq, pol, false)
			assert.NoError(t, err)
			assert.Equal(t, "other-ns", resolvedNamespace)
			assert.NotNil(t, resp)
			assert.Len(t, resp.Policies, 1)
			assert.Nil(t, resp.Policies[0].Result)
		})
	})
}
