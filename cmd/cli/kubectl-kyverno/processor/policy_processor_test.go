package processor

import (
	"os"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var policyNamespaceSelector = []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "enforce-pod-name"
	},
	"spec": {
	  "validationFailureAction": "audit",
	  "background": true,
	  "rules": [
		{
		  "name": "validate-name",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ],
			  "namespaceSelector": {
				"matchExpressions": [
				  {
					"key": "foo.com/managed-state",
					"operator": "In",
					"values": [
					  "managed"
					]
				  }
				]
			  }
			}
		  },
		  "validate": {
			"message": "The Pod must end with -nginx",
			"pattern": {
			  "metadata": {
				"name": "*-nginx"
			  }
			}
		  }
		}
	  ]
	}
  }
`)

func Test_NamespaceSelector(t *testing.T) {
	type TestCase struct {
		policy               []byte
		resource             []byte
		namespaceSelectorMap map[string]map[string]string
		result               ResultCounts
	}

	testcases := []TestCase{
		{
			policy:   policyNamespaceSelector,
			resource: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"test1"},"spec":{"containers":[{"image":"nginx:latest","name":"test-fail"}]}}`),
			namespaceSelectorMap: map[string]map[string]string{
				"test1": {
					"foo.com/managed-state": "managed",
				},
			},
			result: ResultCounts{
				Pass:  0,
				Fail:  1,
				Warn:  0,
				Error: 0,
				Skip:  0,
			},
		},
		{
			policy:   policyNamespaceSelector,
			resource: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-nginx","namespace":"test1"},"spec":{"containers":[{"image":"nginx:latest","name":"test-pass"}]}}`),
			namespaceSelectorMap: map[string]map[string]string{
				"test1": {
					"foo.com/managed-state": "managed",
				},
			},
			result: ResultCounts{
				Pass:  1,
				Fail:  1,
				Warn:  0,
				Error: 0,
				Skip:  0,
			},
		},
	}
	rc := &ResultCounts{}
	for _, tc := range testcases {
		policyArray, _, _, _, _, _, _, _ := yamlutils.GetPolicy(tc.policy)
		resourceArray, _ := resource.GetUnstructuredResources(tc.resource)
		processor := PolicyProcessor{
			Store:                &store.Store{},
			Policies:             policyArray,
			Resource:             *resourceArray[0],
			MutateLogPath:        "",
			UserInfo:             nil,
			NamespaceSelectorMap: tc.namespaceSelectorMap,
			Rc:                   rc,
			Out:                  os.Stdout,
		}
		processor.ApplyPoliciesOnResource()
		assert.Equal(t, int64(rc.Pass), int64(tc.result.Pass))
		assert.Equal(t, int64(rc.Fail), int64(tc.result.Fail))
		assert.Equal(t, int64(rc.Skip), int64(tc.result.Skip))
		assert.Equal(t, int64(rc.Warn), int64(tc.result.Warn))
		assert.Equal(t, int64(rc.Error), int64(tc.result.Error))
	}
}

func Test_resolveResource_fromValidatingPolicy(t *testing.T) {
	// resolveResource should find resource names by scanning ValidatingPolicies
	// when RESTMapper is unavailable in non-cluster CLI test mode.
	mc := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{}
	vp.Spec.MatchConstraints = mc

	p := &PolicyProcessor{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_fromMutatingPolicy(t *testing.T) {
	// resolveResource should find resource names by scanning MutatingPolicies.
	// This is the core fix: before this change resolveResource only checked
	// ValidatingPolicies, so MutatingPolicy tests would fail with
	// "failed to get resource from <Kind>".
	mc := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	mp := &policiesv1beta1.MutatingPolicy{}
	mp.Spec.MatchConstraints = mc

	p := &PolicyProcessor{
		MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{mp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_notFound(t *testing.T) {
	// UnsafeGuessKindToResource always produces a plural form for standard
	// English kind names, so resolveResource never errors for those.
	// With an empty processor it should fall back to the guessed resource.
	p := &PolicyProcessor{}
	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_mutatingPolicyTakesPrecedence(t *testing.T) {
	// When vpol only has pods and mpol has deployments, resolving "Deployment"
	// should fall through to MutatingPolicy.
	mcVpol := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"pods"},
					},
				},
			},
		},
	}
	mcMpol := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{}
	vp.Spec.MatchConstraints = mcVpol
	mp := &policiesv1beta1.MutatingPolicy{}
	mp.Spec.MatchConstraints = mcMpol

	p := &PolicyProcessor{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{mp},
	}

	// "Deployment" should match from MutatingPolicy since vpol only has pods
	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_fromGeneratingPolicy(t *testing.T) {
	// resolveResource should find resource names by scanning GeneratingPolicies.
	// This tests Fix 1: before this change, only ValidatingPolicies and
	// MutatingPolicies were scanned, so GeneratingPolicy tests would fail with
	// "failed to get resource from <Kind>".
	mc := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	gp := &policiesv1beta1.GeneratingPolicy{}
	gp.Spec.MatchConstraints = mc

	p := &PolicyProcessor{
		GeneratingPolicies: []policiesv1beta1.GeneratingPolicyLike{gp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_generatingPolicyFallsThrough(t *testing.T) {
	// When vpol+mpol only have pods, resolving "Deployment" should
	// fall through to GeneratingPolicy which has deployments.
	mcPods := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"pods"},
					},
				},
			},
		},
	}
	mcDeploys := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{}
	vp.Spec.MatchConstraints = mcPods
	mp := &policiesv1beta1.MutatingPolicy{}
	mp.Spec.MatchConstraints = mcPods
	gp := &policiesv1beta1.GeneratingPolicy{}
	gp.Spec.MatchConstraints = mcDeploys

	p := &PolicyProcessor{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{mp},
		GeneratingPolicies: []policiesv1beta1.GeneratingPolicyLike{gp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func TestMakePolicyContext_ConnectOperation(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "check-connect"
		},
		"spec": {
			"rules": [
				{
					"name": "check-connect",
					"match": {
						"all": [
							{
								"resources": {
									"kinds": ["Pod"],
									"operations": ["CONNECT"]
								}
							}
						]
					},
					"validate": {
						"deny": {}
					}
				}
			]
		}
	}`)
	policies, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyRaw)
	assert.NilError(t, err)
	assert.Equal(t, len(policies), 1)

	resourceRaw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod","namespace":"default"}}`)
	resources, err := resource.GetUnstructuredResources(resourceRaw)
	assert.NilError(t, err)

	jp := jmespath.New(config.NewDefaultConfiguration(false))
	cfg := config.NewDefaultConfiguration(false)

	t.Run("explicit operation CONNECT", func(t *testing.T) {
		p := &PolicyProcessor{
			Operation: "CONNECT",
		}
		pCtx, err := p.makePolicyContext(jp, cfg, *resources[0], policies[0], nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, "")
		assert.NilError(t, err)
		assert.Equal(t, kyvernov1.Connect, pCtx.Operation())
		obj, err := pCtx.JSONContext().Query("request.object")
		assert.NilError(t, err)
		assert.Assert(t, obj == nil || obj.(map[string]interface{}) == nil)
		oldObj, err := pCtx.JSONContext().Query("request.oldObject")
		assert.NilError(t, err)
		assert.Assert(t, oldObj == nil || oldObj.(map[string]interface{}) == nil)
		res := pCtx.NewResource()
		assert.Equal(t, res.GetName(), "test-pod")
	})

	t.Run("operation from values request.operation CONNECT", func(t *testing.T) {
		vars, err := variables.New(nil, nil, "", "", &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{
				"request.operation": "CONNECT",
			},
		})
		assert.NilError(t, err)

		p := &PolicyProcessor{
			Variables: vars,
		}
		pCtx, err := p.makePolicyContext(jp, cfg, *resources[0], policies[0], nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, "")
		assert.NilError(t, err)
		assert.Equal(t, kyvernov1.Connect, pCtx.Operation())
		obj, err := pCtx.JSONContext().Query("request.object")
		assert.NilError(t, err)
		assert.Assert(t, obj == nil || obj.(map[string]interface{}) == nil)
		oldObj, err := pCtx.JSONContext().Query("request.oldObject")
		assert.NilError(t, err)
		assert.Assert(t, oldObj == nil || oldObj.(map[string]interface{}) == nil)
		res := pCtx.NewResource()
		assert.Equal(t, res.GetName(), "test-pod")
	})

	t.Run("operation from resource-specific values request.operation CONNECT", func(t *testing.T) {
		vars, err := variables.New(nil, nil, "", "", &v1alpha1.ValuesSpec{
			Policies: []v1alpha1.Policy{
				{
					Name: "check-connect",
					Resources: []v1alpha1.Resource{
						{
							Name: "test-pod",
							Values: map[string]interface{}{
								"request.operation": "CONNECT",
							},
						},
					},
				},
			},
		})
		assert.NilError(t, err)

		p := &PolicyProcessor{
			Variables: vars,
		}
		pCtx, err := p.makePolicyContext(jp, cfg, *resources[0], policies[0], nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, "")
		assert.NilError(t, err)
		assert.Equal(t, kyvernov1.Connect, pCtx.Operation())
		obj, err := pCtx.JSONContext().Query("request.object")
		assert.NilError(t, err)
		assert.Assert(t, obj == nil || obj.(map[string]interface{}) == nil)
		oldObj, err := pCtx.JSONContext().Query("request.oldObject")
		assert.NilError(t, err)
		assert.Assert(t, oldObj == nil || oldObj.(map[string]interface{}) == nil)
		res := pCtx.NewResource()
		assert.Equal(t, res.GetName(), "test-pod")
	})
}

func Test_ConnectOperation_PolicyEvaluation(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "block-connect"
		},
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [
				{
					"name": "check-connect",
					"match": {
						"all": [
							{
								"resources": {
									"kinds": ["Pod"],
									"operations": ["CONNECT"]
								}
							}
						]
					},
					"validate": {
						"message": "CONNECT operation blocked",
						"deny": {}
					}
				}
			]
		}
	}`)
	policies, _, _, _, _, _, _, err := yamlutils.GetPolicy(policyRaw)
	assert.NilError(t, err)

	resourceRaw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod","namespace":"default"}}`)
	resources, err := resource.GetUnstructuredResources(resourceRaw)
	assert.NilError(t, err)

	testCases := []struct {
		name      string
		operation string
		fail      int64
		pass      int64
		skip      int64
	}{
		{
			name:      "CONNECT operation triggers rule and fails",
			operation: "CONNECT",
			fail:      1,
		},
		{
			name: "default CREATE operation does not match CONNECT rule",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rc := &ResultCounts{}
			processor := PolicyProcessor{
				Store:     &store.Store{},
				Policies:  policies,
				Resource:  *resources[0],
				Operation: tc.operation,
				Rc:        rc,
				Out:       os.Stdout,
			}
			_, err := processor.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Equal(t, int64(rc.Fail), tc.fail)
			assert.Equal(t, int64(rc.Pass), tc.pass)
			assert.Equal(t, int64(rc.Skip), tc.skip)
		})
	}
}
