package engine

import (
	"context"
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
)

func TestGetPatches(t *testing.T) {
	t.Run("returns expected patch and policies when resource is mutated", func(t *testing.T) {
		original := &unstructured.Unstructured{}
		original.Object = map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key1": "value1",
			},
		}

		patched := original.DeepCopy()
		data := patched.Object["data"].(map[string]interface{})
		data["key2"] = "value2"

		policy := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sample-policy",
				Namespace: "default",
			},
		}

		ruleResponse := engineapi.NewRuleResponse(
			"add-key2",
			engineapi.Mutation,
			"added key2 to configmap",
			engineapi.RuleStatusPass,
			map[string]string{"source": "mutation"},
		)

		er := EngineResponse{
			Resource:        original,
			PatchedResource: patched,
			Policies: []MutatingPolicyResponse{
				{
					Policy: policy,
					Rules:  []engineapi.RuleResponse{*ruleResponse},
				},
			},
		}

		patches := er.GetPatches()

		assert.NotNil(t, patches)
		assert.Len(t, patches, 1)
		assert.Equal(t, "add", patches[0].Operation)
		assert.Equal(t, "/data/key2", patches[0].Path)
		assert.Equal(t, "value2", patches[0].Value)

		assert.Len(t, er.Policies, 1)
		assert.Equal(t, "sample-policy", er.Policies[0].Policy.GetName())
		assert.Len(t, er.Policies[0].Rules, 1)
	})

	t.Run("returns nil when jsonpatch.CreatePatch fails due to invalid input", func(t *testing.T) {
		badOriginal := &unstructured.Unstructured{}
		badOriginal.Object = map[string]interface{}{
			"key": json.RawMessage("invalid"),
		}
		badPatched := &unstructured.Unstructured{}
		badPatched.Object = map[string]interface{}{
			"key": func() {},
		}

		er := EngineResponse{
			Resource:        badOriginal,
			PatchedResource: badPatched,
		}

		patches := er.GetPatches()
		assert.Nil(t, patches)
	})

	t.Run("returns nil when Resource.MarshalJSON fails", func(t *testing.T) {
		res := &unstructured.Unstructured{}
		res.Object = map[string]interface{}{
			"noncodeable": func() {},
		}

		patched := &unstructured.Unstructured{}

		er := EngineResponse{
			Resource:        res,
			PatchedResource: patched,
		}
		patches := er.GetPatches()
		assert.Nil(t, patches)
	})

	t.Run("returns nil when PatchedResource.MarshalJSON fails", func(t *testing.T) {
		res := &unstructured.Unstructured{}

		patched := &unstructured.Unstructured{}
		patched.Object = map[string]interface{}{
			"noncodeable": func() {},
		}

		er := EngineResponse{
			Resource:        res,
			PatchedResource: patched,
		}

		patches := er.GetPatches()

		assert.Nil(t, patches)
	})
}

type mockAttributes struct{}

func (m *mockAttributes) GetName() string      { return "" }
func (m *mockAttributes) GetNamespace() string { return "default" }
func (m *mockAttributes) GetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}
func (m *mockAttributes) GetSubresource() string { return "" }
func (m *mockAttributes) GetOperation() admission.Operation {
	return admission.Create
}
func (m *mockAttributes) GetOperationOptions() runtime.Object { return nil }
func (m *mockAttributes) IsDryRun() bool                      { return false }
func (m *mockAttributes) GetObject() runtime.Object {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{},
		},
	}
}
func (m *mockAttributes) GetOldObject() runtime.Object { return nil }
func (m *mockAttributes) GetKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
}
func (m *mockAttributes) GetUserInfo() user.Info                { return &user.DefaultInfo{} }
func (m *mockAttributes) AddAnnotation(key, value string) error { return nil }
func (m *mockAttributes) AddAnnotationWithLevel(key, value string, level auditinternal.Level) error {
	return nil
}
func (m *mockAttributes) GetReinvocationContext() admission.ReinvocationContext { return nil }

var (
	mapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{
			Group:   "apps",
			Version: "v1",
		},
	})

	ctx        = context.Background()
	res        = unstructured.Unstructured{}
	matcher    = matching.NewMatcher()
	nsResolver = func(ns string) *corev1.Namespace {
		return nil
	}
	predicate     = func(p policiesv1beta1.MutatingPolicyLike) bool { return true }
	typeConverter = compiler.NewStaticTypeConverterManager(openapi.NewClient(&rest.RESTClient{}))
)

type mockFailingProvider struct{}

func (m *mockFailingProvider) Fetch(ctx context.Context, mutate bool) []Policy {
	return nil
}
func (m *mockFailingProvider) MatchesMutateExisting(context.Context, admission.Attributes, *corev1.Namespace) []string {
	return nil
}

type fakeTypeConverter struct{}

func (f *fakeTypeConverter) GetTypeConverter(gvk schema.GroupVersionKind) managedfields.TypeConverter {
	return managedfields.NewDeducedTypeConverter()
}

func TestEvaluate(t *testing.T) {
	t.Run("no policies and no exceptions returns empty response without error", func(t *testing.T) {
		pols := []policiesv1beta1.MutatingPolicyLike{}
		polexs := []*policiesv1beta1.PolicyException{}

		provider, err := NewProvider(compiler.NewCompiler(), pols, polexs)

		assert.NoError(t, err)
		engine := NewEngine(provider, nsResolver, matcher, typeConverter, &libs.FakeContextProvider{})
		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})

	t.Run("provider returns an empty response", func(t *testing.T) {
		engine := NewEngine(&mockFailingProvider{}, nsResolver, matcher, typeConverter, &libs.FakeContextProvider{})
		resp, _ := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)
		assert.Equal(t, EngineResponse{}, resp)
	})

	t.Run("successful match and mutation with mutateExisting enabled", func(t *testing.T) {
		mutateExisting := true
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "add-label",
			},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
						Enabled: &mutateExisting,
					},
				},
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
								},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{metadata: Object.metadata{labels: {"env": "test"}}}`,
						},
					},
				},
			},
		}

		pols := []policiesv1beta1.MutatingPolicyLike{mpol}

		provider, err := NewProvider(compiler.NewCompiler(), pols, nil)

		assert.NoError(t, err)
		engine := NewEngine(
			provider,
			func(ns string) *corev1.Namespace {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
			}, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})
		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})

	t.Run("multiple policies chain mutations correctly in Evaluate", func(t *testing.T) {
		mutateExisting := true
		mpol1 := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "add-label-env",
			},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
						Enabled: &mutateExisting,
					},
				},
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
								},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{metadata: Object.metadata{labels: {"env": "staging"}}}`,
						},
					},
				},
			},
		}

		mpol2 := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "add-label-team",
			},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
						Enabled: &mutateExisting,
					},
				},
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{"CREATE"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
								},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{metadata: Object.metadata{labels: {"team": "backend"}}}`,
						},
					},
				},
			},
		}

		pols := []policiesv1beta1.MutatingPolicyLike{mpol1, mpol2}

		provider, err := NewProvider(compiler.NewCompiler(), pols, nil)
		assert.NoError(t, err)

		engine := NewEngine(
			provider,
			func(ns string) *corev1.Namespace {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
			}, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})

		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.PatchedResource)
		assert.Len(t, resp.Policies, 2)

		// Verify that both mutations were applied (chained correctly)
		labels, found, _ := unstructured.NestedMap(resp.PatchedResource.Object, "metadata", "labels")
		assert.True(t, found, "expected labels to be present in patched resource")
		assert.Equal(t, "staging", labels["env"], "first policy mutation should be present")
		assert.Equal(t, "backend", labels["team"], "second policy mutation should be present")
	})
}

func TestHandle(t *testing.T) {
	tests := []struct {
		name           string
		policies       []policiesv1beta1.MutatingPolicyLike
		requestObject  string
		kind           string
		matchNamespace string
		predicate      Predicate
		expectPolicies int
		expectPatched  bool
		expectLabel    string
		expectLabels   map[string]string // for multiple label checks
	}{
		{
			name: "Successful match and mutation",
			policies: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "add-label",
					},
					Spec: policiesv1beta1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{"CREATE"},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Mutations: []admissionregistrationv1alpha1.Mutation{
							{
								PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
								ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
									Expression: `Object{metadata: Object.metadata{labels: {"env": "test"}}}`,
								},
							},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.MutatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectPatched:  true,
			expectLabel:    "test",
		},
		{
			name: "predicate returns false",
			policies: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "skip-policy"},
					Spec:       policiesv1beta1.MutatingPolicySpec{},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.MutatingPolicyLike) bool { return false },
			expectPolicies: 0,
			expectPatched:  false,
		},
		{
			name: "no mutation specified",
			policies: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "no-mutation"},
					Spec: policiesv1beta1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									ResourceNames: []string{"Deployment"},
								},
							},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.MutatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectPatched:  false,
		},
		{
			name: "Multiple policies chain mutations correctly",
			policies: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "add-label-env",
					},
					Spec: policiesv1beta1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{"CREATE"},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Mutations: []admissionregistrationv1alpha1.Mutation{
							{
								PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
								ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
									Expression: `Object{metadata: Object.metadata{labels: {"env": "production"}}}`,
								},
							},
						},
					},
				},
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "add-label-team",
					},
					Spec: policiesv1beta1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{"CREATE"},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Mutations: []admissionregistrationv1alpha1.Mutation{
							{
								PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
								ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
									Expression: `Object{metadata: Object.metadata{labels: {"team": "platform"}}}`,
								},
							},
						},
					},
				},
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "add-label-version",
					},
					Spec: policiesv1beta1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{"CREATE"},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Mutations: []admissionregistrationv1alpha1.Mutation{
							{
								PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
								ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
									Expression: `Object{metadata: Object.metadata{labels: {"version": "v1"}}}`,
								},
							},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.MutatingPolicyLike) bool { return true },
			expectPolicies: 3,
			expectPatched:  true,
			expectLabels: map[string]string{
				"env":     "production",
				"team":    "platform",
				"version": "v1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Compile policies
			provider, err := NewProvider(
				compiler.NewCompiler(),
				tc.policies,
				nil,
			)
			assert.NoError(t, err)

			// Create engine
			eng := NewEngine(
				provider,
				func(ns string) *corev1.Namespace {
					return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				},
				matching.NewMatcher(),
				&fakeTypeConverter{},
				&libs.FakeContextProvider{},
			)

			dryRun := true

			// Prepare admission request
			req := engine.EngineRequest{
				Request: admissionv1.AdmissionRequest{
					Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: tc.kind},
					Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
					Namespace: tc.matchNamespace,
					Name:      "nginx",
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: []byte(tc.requestObject),
					},
					OldObject: runtime.RawExtension{
						Raw: []byte(tc.requestObject),
					},
					DryRun: &dryRun,
				},
			}

			// Run Handle
			resp, err := eng.Handle(context.Background(), req, tc.predicate)
			assert.NoError(t, err)

			// Assertions
			assert.Len(t, resp.Policies, tc.expectPolicies)

			if tc.expectPatched {
				assert.NotNil(t, resp.PatchedResource)

				if tc.expectLabel != "" {
					labels, found, _ := unstructured.NestedMap(resp.PatchedResource.Object, "metadata", "labels")
					assert.True(t, found)
					assert.Equal(t, tc.expectLabel, labels["env"])
				}
				if tc.expectLabels != nil {
					labels, found, _ := unstructured.NestedMap(resp.PatchedResource.Object, "metadata", "labels")
					assert.True(t, found, "expected labels to be present in patched resource")
					for key, expectedValue := range tc.expectLabels {
						assert.Equal(t, expectedValue, labels[key], "label %s should have value %s", key, expectedValue)
					}
				}
			} else {
				assert.Nil(t, resp.PatchedResource)
			}
		})
	}
}

func TestMatchedMutateExistingPolicies(t *testing.T) {
	t.Run("valid object raw", func(t *testing.T) {
		dryRun := true
		req := engine.EngineRequest{
			Request: admissionv1.AdmissionRequest{
				Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				Namespace: "default",
				Name:      "test-deploy",
				Operation: admissionv1.Create,
				Object: runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment"}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment"}`),
				},
				DryRun: &dryRun,
			},
		}

		pols := []policiesv1beta1.MutatingPolicyLike{}
		polexs := []*policiesv1beta1.PolicyException{}

		provider, _ := NewProvider(compiler.NewCompiler(), pols, polexs)

		eng := NewEngine(provider, nsResolver, matcher, typeConverter, &libs.FakeContextProvider{})

		resp := eng.MatchedMutateExistingPolicies(ctx, req)

		assert.NotNil(t, resp)
	})

	t.Run("invalid object raw", func(t *testing.T) {
		dryRun := true
		req := engine.EngineRequest{
			Request: admissionv1.AdmissionRequest{
				Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				Namespace: "default",
				Name:      "test-deploy",
				Operation: admissionv1.Create,
				Object: runtime.RawExtension{
					Raw: []byte(`{invalid-json}`),
				},
				DryRun: &dryRun,
			},
		}

		pols := []policiesv1beta1.MutatingPolicyLike{}
		polexs := []*policiesv1beta1.PolicyException{}
		provider, _ := NewProvider(compiler.NewCompiler(), pols, polexs)

		eng := NewEngine(provider, nsResolver, matcher, typeConverter, &libs.FakeContextProvider{})

		resp := eng.MatchedMutateExistingPolicies(ctx, req)

		assert.Nil(t, resp)
	})
}
