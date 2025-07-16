package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
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

		policy := &policiesv1alpha1.MutatingPolicy{
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
		assert.Equal(t, "sample-policy", er.Policies[0].Policy.Name)
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

// mock Context
type fakeContext struct{}

func (f *fakeContext) GenerateResources(string, []map[string]any) error        { return nil }
func (f *fakeContext) GetGlobalReference(name, projection string) (any, error) { return name, nil }
func (f *fakeContext) GetImageData(image string) (map[string]any, error) {
	return map[string]any{"test": image}, nil
}
func (f *fakeContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{}, nil
}
func (f *fakeContext) GetGeneratedResources() []*unstructured.Unstructured { return nil }
func (f *fakeContext) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ClearGeneratedResources() {}
func (f *fakeContext) SetGenerateContext(polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool) {
	panic("not implemented")
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
	predicate     = func(p policiesv1alpha1.MutatingPolicy) bool { return true }
	typeConverter = compiler.NewStaticTypeConverterManager(openapi.NewClient(&rest.RESTClient{}))
)

type mockFailingProvider struct{}

func (m *mockFailingProvider) Fetch(ctx context.Context, mutate bool) ([]Policy, error) {
	return nil, errors.New("fetch failed")
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
		pols := []policiesv1alpha1.MutatingPolicy{}
		polexs := []*policiesv1alpha1.PolicyException{}

		provider, err := NewProvider(compiler.NewCompiler(), pols, polexs)

		assert.NoError(t, err)
		engine := NewEngine(provider, nsResolver, matcher, typeConverter, &fakeContext{})
		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})

	t.Run("provider fetch failure returns error and empty response", func(t *testing.T) {
		engine := NewEngine(&mockFailingProvider{}, nsResolver, matcher, typeConverter, &fakeContext{})
		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.Error(t, err)
		assert.Equal(t, EngineResponse{}, resp)
	})

	t.Run("successful match and mutation with mutateExisting enabled", func(t *testing.T) {
		mutateExisting := true
		mpol := policiesv1alpha1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "add-label",
			},
			Spec: policiesv1alpha1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
						Enabled: &mutateExisting,
					},
				},
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
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

		pols := []policiesv1alpha1.MutatingPolicy{mpol}

		provider, err := NewProvider(compiler.NewCompiler(), pols, nil)

		assert.NoError(t, err)
		engine := NewEngine(
			provider,
			func(ns string) *corev1.Namespace {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
			}, matcher, &fakeTypeConverter{}, &fakeContext{})
		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})
}

func TestHandle(t *testing.T) {
	tests := []struct {
		name           string
		policies       []policiesv1alpha1.MutatingPolicy
		requestObject  string
		kind           string
		matchNamespace string
		predicate      Predicate
		expectPolicies int
		expectPatched  bool
		expectLabel    string
	}{
		{
			name: "Successful match and mutation",
			policies: []policiesv1alpha1.MutatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "add-label",
					},
					Spec: policiesv1alpha1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
							ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
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
			predicate:      func(p policiesv1alpha1.MutatingPolicy) bool { return true },
			expectPolicies: 1,
			expectPatched:  true,
			expectLabel:    "test",
		},
		{
			name: "predicate returns false",
			policies: []policiesv1alpha1.MutatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "skip-policy"},
					Spec:       policiesv1alpha1.MutatingPolicySpec{},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1alpha1.MutatingPolicy) bool { return false },
			expectPolicies: 0,
			expectPatched:  false,
		},
		{
			name: "no mutation specified",
			policies: []policiesv1alpha1.MutatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "no-mutation"},
					Spec: policiesv1alpha1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
							ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
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
			predicate:      func(p policiesv1alpha1.MutatingPolicy) bool { return true },
			expectPolicies: 1,
			expectPatched:  false,
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
				&fakeContext{},
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

		pols := []policiesv1alpha1.MutatingPolicy{}
		polexs := []*policiesv1alpha1.PolicyException{}

		provider, _ := NewProvider(compiler.NewCompiler(), pols, polexs)

		eng := NewEngine(provider, nsResolver, matcher, typeConverter, &fakeContext{})

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

		pols := []policiesv1alpha1.MutatingPolicy{}
		polexs := []*policiesv1alpha1.PolicyException{}
		provider, _ := NewProvider(compiler.NewCompiler(), pols, polexs)

		eng := NewEngine(provider, nsResolver, matcher, typeConverter, &fakeContext{})

		resp := eng.MatchedMutateExistingPolicies(ctx, req)

		assert.Nil(t, resp)
	})
}
