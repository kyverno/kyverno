package engine

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
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

func TestHandle(t *testing.T) {
	t.Run("should handle policy with match constraints and return response", func(t *testing.T) {
		gpol := &policiesv1beta1.GeneratingPolicy{
			Spec: policiesv1beta1.GeneratingPolicySpec{
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
		}
		pol := Policy{
			Policy: gpol,
		}

		resp, err := eng.Handle(req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should return error when ExtractResources fails due to empty request", func(t *testing.T) {
		req := engine.EngineRequest{
			Request: admissionv1.AdmissionRequest{},
		}

		resp, err := eng.Handle(req, Policy{}, false)
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
		gpol := &policiesv1beta1.GeneratingPolicy{
			Spec: policiesv1beta1.GeneratingPolicySpec{
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
		}
		pol := Policy{
			Policy: gpol,
		}

		resp, err := eng.Handle(req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate policy with valid match condition on default namespace", func(t *testing.T) {
		gpol := &policiesv1beta1.GeneratingPolicy{
			Spec: policiesv1beta1.GeneratingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "valid-namespace",
						Expression: "object.metadata.namespace == 'default'",
					},
				},
			},
		}
		pol := Policy{
			Policy: gpol,
		}

		resp, err := eng.Handle(req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate policy with match condition but different namespace", func(t *testing.T) {
		resource.SetName("valid")
		resource.SetNamespace("valid-ns")

		gpol := &policiesv1beta1.GeneratingPolicy{
			Spec: policiesv1beta1.GeneratingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "valid-namespace",
						Expression: "object.metadata.namespace == 'default'",
					},
				},
			},
		}
		pol := Policy{
			Policy: gpol,
		}

		resp, err := eng.Handle(req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate compiled policy without exceptions", func(t *testing.T) {
		gpol := &policiesv1beta1.GeneratingPolicy{
			Spec: policiesv1beta1.GeneratingPolicySpec{
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
		resp, err := eng.Handle(req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("should evaluate compiled policy with variable expressions and policy exceptions", func(t *testing.T) {
		obj.SetNamespace("default")
		gpol := &policiesv1beta1.GeneratingPolicy{
			Spec: policiesv1beta1.GeneratingPolicySpec{
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
				Generation: []policiesv1beta1.Generation{
					{
						Expression: "generator.Apply(variables.nsName, variables.nsName)",
					},
				},
			},
		}
		exceptions := []*policiesv1alpha1.PolicyException{
			{
				Spec: policiesv1alpha1.PolicyExceptionSpec{
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
		resp, err := eng.Handle(req, pol, false)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}
