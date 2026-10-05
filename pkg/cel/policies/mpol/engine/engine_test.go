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

func (m *mockFailingProvider) MatchesMutateExisting(context.Context, admission.Attributes, *admissionv1.AdmissionRequest, *corev1.Namespace, func() (map[string]any, error)) []string {
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

		provider, err := NewProvider(compiler.NewCompiler(), pols, polexs, libs.NewFakeContextProvider())

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

		provider, err := NewProvider(compiler.NewCompiler(), pols, nil, libs.NewFakeContextProvider())

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

	t.Run("audit annotations surface as rule response properties", func(t *testing.T) {
		mutateExisting := true
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "add-label-with-annotations",
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
				AuditAnnotations: []admissionregistrationv1.AuditAnnotation{
					{Key: "resource-name", ValueExpression: `'name/' + object.metadata.name`},
					{Key: "empty-omitted", ValueExpression: `''`},
				},
			},
		}

		provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{mpol}, nil, libs.NewFakeContextProvider())
		assert.NoError(t, err)
		engine := NewEngine(
			provider,
			func(ns string) *corev1.Namespace {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
			}, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})
		resp, err := engine.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NoError(t, err)
		if assert.Len(t, resp.Policies, 1) && assert.Len(t, resp.Policies[0].Rules, 1) {
			rule := resp.Policies[0].Rules[0]
			assert.Equal(t, engineapi.RuleStatusPass, rule.Status())
			assert.Equal(t, map[string]string{"resource-name": "name/nginx"}, rule.Properties())
		}
	})

	t.Run("matches target constraints with trigger variables", func(t *testing.T) {
		mutateExisting := true
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "different-trigger-target"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: &mutateExisting},
				},
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"},
							},
						},
					}},
				},
				Variables: []admissionregistrationv1.Variable{{Name: "triggerName", Expression: "request.name"}},
				TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
					MatchResources: admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
								Rule: admissionregistrationv1.Rule{
									APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"},
								},
							},
						}},
					},
				},
				TargetMatchConditions: []admissionregistrationv1.MatchCondition{{
					Name: "trigger-and-target", Expression: `variables.triggerName == "trigger" && object.metadata.name == "target"`,
				}},
				Mutations: []admissionregistrationv1alpha1.Mutation{{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: `Object{metadata: Object.metadata{labels: {"mutated": "true"}}}`,
					},
				}},
			},
		}
		provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{mpol}, nil, libs.NewFakeContextProvider())
		if !assert.NoError(t, err) {
			return
		}
		target := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "target", "namespace": "default"},
		}}
		attr := admission.NewAttributesRecord(
			target, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			"default", "target", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			"", admission.Update, nil, false, &user.DefaultInfo{},
		)
		request := admissionv1.AdmissionRequest{Operation: admissionv1.Update, Name: "trigger", Namespace: "default"}
		eng := NewEngine(provider, nsResolver, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})

		response, err := eng.Evaluate(ctx, attr, request, predicate)

		assert.NoError(t, err)
		if assert.NotNil(t, response.PatchedResource) {
			assert.Equal(t, "true", response.PatchedResource.GetLabels()["mutated"])
		}
	})

	t.Run("matches expression-only target constraints with a target kind different from the trigger", func(t *testing.T) {
		mutateExisting := true
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "expression-only-target"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: &mutateExisting},
				},
				// The trigger's matchConstraints only cover deployments; the target
				// resolved via the expression below is a ConfigMap. Because
				// targetMatchConstraints has no resourceRules of its own, the
				// matcher gate must not fall back to the trigger's matchConstraints
				// (a different kind) or the target would be rejected before its
				// targetMatchConditions ever run.
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"},
							},
						},
					}},
				},
				TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
					Expression: `resource.get("v1", "configmaps", "default", "target")`,
				},
				TargetMatchConditions: []admissionregistrationv1.MatchCondition{{
					Name: "target-name", Expression: `object.metadata.name == "target"`,
				}},
				Mutations: []admissionregistrationv1alpha1.Mutation{{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: `Object{metadata: Object.metadata{labels: {"mutated": "true"}}}`,
					},
				}},
			},
		}
		provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{mpol}, nil, libs.NewFakeContextProvider())
		if !assert.NoError(t, err) {
			return
		}
		target := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "target", "namespace": "default"},
		}}
		attr := admission.NewAttributesRecord(
			target, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			"default", "target", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			"", admission.Update, nil, false, &user.DefaultInfo{},
		)
		request := admissionv1.AdmissionRequest{Operation: admissionv1.Update, Name: "target", Namespace: "default"}
		eng := NewEngine(provider, nsResolver, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})

		response, err := eng.Evaluate(ctx, attr, request, predicate)

		assert.NoError(t, err)
		if assert.NotNil(t, response.PatchedResource) {
			assert.Equal(t, "true", response.PatchedResource.GetLabels()["mutated"])
		}
	})

	t.Run("respects matchConditions during background mutateExisting evaluation when no explicit target is defined", func(t *testing.T) {
		mutateExisting := true
		newPolicy := func() *policiesv1beta1.MutatingPolicy {
			return &policiesv1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "conditional-mutate-existing"},
				Spec: policiesv1beta1.MutatingPolicySpec{
					EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
						MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: &mutateExisting},
					},
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
								Rule: admissionregistrationv1.Rule{
									APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"},
								},
							},
						}},
					},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "require-enabled-annotation",
						Expression: `object.metadata.?annotations[?'enabled'].orValue('false') == 'true'`,
					}},
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{metadata: Object.metadata{labels: {"mutated": "true"}}}`,
						},
					}},
				},
			}
		}

		newAttr := func(annotations map[string]interface{}) admission.Attributes {
			metadata := map[string]interface{}{"name": "cm", "namespace": "default"}
			if annotations != nil {
				metadata["annotations"] = annotations
			}
			obj := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "ConfigMap", "metadata": metadata,
			}}
			return admission.NewAttributesRecord(
				obj, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
				"default", "cm", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
				"", admission.Update, nil, false, &user.DefaultInfo{},
			)
		}
		nsResolver := func(ns string) *corev1.Namespace {
			return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		}
		req := admissionv1.AdmissionRequest{Operation: admissionv1.Update, Namespace: "default", Name: "cm"}

		// The resource does not satisfy spec.matchConditions: mutateExisting/background
		// evaluation must skip it just like admission-time evaluation would.
		providerSkip, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{newPolicy()}, nil, libs.NewFakeContextProvider())
		if !assert.NoError(t, err) {
			return
		}
		engSkip := NewEngine(providerSkip, nsResolver, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})
		respSkip, err := engSkip.Evaluate(ctx, newAttr(nil), req, predicate)
		assert.NoError(t, err)
		assert.Nil(t, respSkip.PatchedResource, "matchConditions should have excluded this resource from mutateExisting")

		// The resource satisfies spec.matchConditions: it should be mutated.
		providerMatch, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{newPolicy()}, nil, libs.NewFakeContextProvider())
		if !assert.NoError(t, err) {
			return
		}
		engMatch := NewEngine(providerMatch, nsResolver, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})
		respMatch, err := engMatch.Evaluate(ctx, newAttr(map[string]interface{}{"enabled": "true"}), req, predicate)
		assert.NoError(t, err)
		if assert.NotNil(t, respMatch.PatchedResource) {
			assert.Equal(t, "true", respMatch.PatchedResource.GetLabels()["mutated"])
		}
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

		provider, err := NewProvider(compiler.NewCompiler(), pols, nil, libs.NewFakeContextProvider())
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

	// Regression test for https://github.com/kyverno/kyverno/issues/16953:
	// Evaluate() must resolve the namespace via nsResolver so that namespaceSelector
	// in matchConstraints is correctly evaluated during mutate-existing background scans.
	t.Run("Evaluate respects namespaceSelector via nsResolver", func(t *testing.T) {
		mutateExisting := true
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "add-label-production-only",
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
								Operations: []admissionregistrationv1.OperationType{"*"},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"configmaps"},
								},
							},
						},
					},
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "production"},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{metadata: Object.metadata{labels: {"mutated": "true"}}}`,
						},
					},
				},
			},
		}

		provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{mpol}, nil, libs.NewFakeContextProvider())
		if !assert.NoError(t, err) {
			return
		}

		target := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]interface{}{"name": "cm", "namespace": "production"},
		}}
		attr := admission.NewAttributesRecord(
			target, nil, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			"production", "cm", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			"", admission.Update, nil, false, &user.DefaultInfo{},
		)
		request := admissionv1.AdmissionRequest{Operation: admissionv1.Update, Name: "cm", Namespace: "production"}

		// Sub-test 1: nsResolver returns a namespace with label env=production → policy should apply.
		t.Run("applies mutation when namespace label matches selector", func(t *testing.T) {
			nsRes := func(ns string) *corev1.Namespace {
				return &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   ns,
						Labels: map[string]string{"env": "production"},
					},
				}
			}
			eng := NewEngine(provider, nsRes, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})
			resp, err := eng.Evaluate(ctx, attr, request, predicate)
			assert.NoError(t, err)
			if assert.NotNil(t, resp.PatchedResource, "expected mutation to be applied for matching namespace") {
				assert.Equal(t, "true", resp.PatchedResource.GetLabels()["mutated"])
			}
		})

		// Sub-test 2: nsResolver returns a namespace WITHOUT the required label → policy should not apply.
		t.Run("skips mutation when namespace label does not match selector", func(t *testing.T) {
			nsRes := func(ns string) *corev1.Namespace {
				return &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   ns,
						Labels: map[string]string{"env": "staging"},
					},
				}
			}
			eng := NewEngine(provider, nsRes, matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{})
			resp, err := eng.Evaluate(ctx, attr, request, predicate)
			assert.NoError(t, err)
			assert.Nil(t, resp.PatchedResource, "expected no mutation for non-matching namespace")
		})
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
				libs.NewFakeContextProvider(),
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

		provider, _ := NewProvider(compiler.NewCompiler(), pols, polexs, libs.NewFakeContextProvider())

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
		provider, _ := NewProvider(compiler.NewCompiler(), pols, polexs, libs.NewFakeContextProvider())

		eng := NewEngine(provider, nsResolver, matcher, typeConverter, &libs.FakeContextProvider{})

		resp := eng.MatchedMutateExistingPolicies(ctx, req)

		assert.Nil(t, resp)
	})
}

// TestEvaluate_NilMatcherSkipsNamespaceSelector is a regression test for
// https://github.com/kyverno/kyverno/issues/16953.
//
// This test isolates the NamespaceSelector path specifically by using a policy
// whose ResourceRules DO match the attr (apps/v1/deployments + CREATE), so any
// filtering is purely caused by the NamespaceSelector. When the engine is
// constructed with a nil matcher the matchConstraints block is skipped entirely,
// so the namespace label is never checked and the mutation runs. After the fix,
// the real matcher rejects the policy because the resolved namespace lacks the
// required label.
func TestEvaluate_NilMatcherSkipsNamespaceSelector(t *testing.T) {
	mutateExisting := true

	// Policy whose ResourceRules match mockAttributes (apps/v1/deployments CREATE)
	// but whose NamespaceSelector requires "env=production". The resolved namespace
	// ("default") does NOT carry that label, so a real matcher must filter the policy
	// out. Using OperationAll so that only the selector, not the operation, decides
	// the outcome.
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-scoped"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &mutateExisting,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"env": "production"},
				},
				// ResourceRules: use wildcard (*) so the resource-rule check always
				// passes and the selector is the only reason for filtering.
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{"*"},
						},
					},
				}},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{metadata: Object.metadata{labels: {"injected": "true"}}}`,
				},
			}},
		},
	}

	pols := []policiesv1beta1.MutatingPolicyLike{mpol}
	provider, err := NewProvider(compiler.NewCompiler(), pols, nil, libs.NewFakeContextProvider())
	assert.NoError(t, err)

	// Namespace "default" does NOT have the "env=production" label.
	nsNoLabel := func(ns string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	}
	// Namespace "default" WITH the "env=production" label.
	nsWithLabel := func(ns string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   ns,
			Labels: map[string]string{"env": "production"},
		}}
	}

	t.Run("nil matcher skips selector — policy evaluates regardless", func(t *testing.T) {
		eng := NewEngine(provider, nsNoLabel, nil, &fakeTypeConverter{}, &libs.FakeContextProvider{})
		resp, err := eng.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NoError(t, err)
		// Nil matcher skips the entire matchConstraints block, so namespaceSelector
		// is never checked. The policy must reach CEL evaluation and produce a Pass rule.
		if assert.Len(t, resp.Policies, 1) {
			if assert.Len(t, resp.Policies[0].Rules, 1) {
				assert.Equal(t, engineapi.RuleStatusPass, resp.Policies[0].Rules[0].Status(),
					"nil matcher: policy must evaluate (CEL mutation should pass)")
			}
		}
	})

	t.Run("real matcher filters when namespace lacks required label", func(t *testing.T) {
		eng := NewEngine(provider, nsNoLabel, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})
		resp, err := eng.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NoError(t, err)
		// Real matcher evaluates the namespaceSelector. The resolved namespace
		// has no "env=production" label, so handlePolicy must return early.
		if assert.Len(t, resp.Policies, 1) {
			assert.Empty(t, resp.Policies[0].Rules,
				"real matcher: policy must be filtered out when namespaceSelector doesn't match")
		}
	})

	t.Run("real matcher passes when namespace carries required label", func(t *testing.T) {
		eng := NewEngine(provider, nsWithLabel, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})
		resp, err := eng.Evaluate(ctx, &mockAttributes{}, admissionv1.AdmissionRequest{}, predicate)

		assert.NoError(t, err)
		// Namespace now has the required label — the selector must pass and the
		// policy must evaluate successfully.
		if assert.Len(t, resp.Policies, 1) {
			if assert.Len(t, resp.Policies[0].Rules, 1) {
				assert.Equal(t, engineapi.RuleStatusPass, resp.Policies[0].Rules[0].Status(),
					"real matcher: policy must evaluate when namespaceSelector matches")
			}
		}
	})
}

// TestEvaluate_TargetOperationNormalization is a regression test for the
// operation-mismatch bug described in Copilot review comments
// discussion_r4178758568 / discussion_r4178758589 / discussion_r4178758602.
//
// When handlePolicy evaluates a target resource, the attr carries an artificial
// operation: the background controller synthesises Update for every scan, and the
// CLI uses an empty "". Without normalization, a targetMatchConstraints whose
// ResourceRules only list CREATE would be rejected by the matcher even though the
// target was already selected for mutation.
//
// The fix normalises all target ResourceRules operations to OperationAll before
// calling matcher.Match, so resource-type and selector evaluation still occur but
// operation filtering does not.
func TestEvaluate_TargetOperationNormalization(t *testing.T) {
	mutateExisting := true

	// Policy: trigger = configmaps/CREATE, target = deployments with CREATE-only rule.
	// The target attr will carry an empty operation (""), simulating the CLI path.
	// Without the fix the matcher would reject the target because "" ∉ {CREATE}.
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "op-norm"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &mutateExisting,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				}},
			},
			// targetMatchConstraints with CREATE-only rule for deployments.
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					}},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{metadata: Object.metadata{labels: {"mutated": "true"}}}`,
				},
			}},
		},
	}

	pols := []policiesv1beta1.MutatingPolicyLike{mpol}
	provider, err := NewProvider(compiler.NewCompiler(), pols, nil, libs.NewFakeContextProvider())
	assert.NoError(t, err)

	nsResolver := func(ns string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	}

	// target attr simulates a deployment with an empty operation (""), as
	// the CLI policy_processor builds it for mutateExisting targets.
	targetDeploy := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "nginx", "namespace": "default"},
	}}
	attrEmptyOp := admission.NewAttributesRecord(
		targetDeploy, nil,
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		"default", "nginx",
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		"", admission.Operation("") /* CLI path: empty operation */, nil, false, &user.DefaultInfo{},
	)

	// target attr simulates the background controller's synthetic Update.
	attrUpdateOp := admission.NewAttributesRecord(
		targetDeploy, nil,
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		"default", "nginx",
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		"", admission.Update /* background path: synthetic Update */, nil, false, &user.DefaultInfo{},
	)

	for _, tc := range []struct {
		name string
		attr admission.Attributes
	}{
		{"CLI empty operation", attrEmptyOp},
		{"background synthetic Update", attrUpdateOp},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eng := NewEngine(provider, nsResolver, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})
			resp, err := eng.Evaluate(ctx, tc.attr, admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Name:      "nginx",
				Namespace: "default",
			}, predicate)

			assert.NoError(t, err)
			// The target's CREATE-only resourceRule must NOT block evaluation when the
			// operation in attr is different (empty or Update). OperationAll normalisation
			// ensures the rule still matches and the mutation is applied.
			if assert.Len(t, resp.Policies, 1) {
				if assert.NotEmpty(t, resp.Policies[0].Rules,
					"target CREATE-only rule must not be filtered out by operation mismatch") {
					assert.Equal(t, engineapi.RuleStatusPass, resp.Policies[0].Rules[0].Status(),
						"mutation must succeed after operation normalisation")
				}
			}
		})
	}
}

// TestEvaluate_ExpressionOnlyTargetNotFilteredByMatcher is a regression test
// for the case where targetMatchConstraints uses an expression (e.g.
// resource.get(...)) with no resourceRules. Previously, wiring a real matcher
// caused handlePolicy to fall back to the trigger's matchConstraints, which
// would incorrectly filter out the resolved target (e.g. a ConfigMap) because
// it doesn't match the trigger resource (e.g. secrets/CREATE).
//
// The fix skips constraint matching when targetMatchConstraints has an
// expression but no resourceRules, since the expression itself resolves
// the target set.
func TestEvaluate_ExpressionOnlyTargetNotFilteredByMatcher(t *testing.T) {
	mutateExisting := true

	// Policy: trigger = secrets/CREATE, target = expression-only (no resourceRules).
	// This mirrors the conformance test at
	// test/conformance/chainsaw/mutating-policies/existing/expression/get/policy.yaml
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "expression-target"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &mutateExisting,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"secrets"},
						},
					},
				}},
			},
			// Expression-only targetMatchConstraints — no ResourceRules.
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				Expression: `resource.get("v1", "configmaps", object.metadata.namespace, "test-cm")`,
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{metadata: Object.metadata{labels: {"patched": "yes"}}}`,
				},
			}},
		},
	}

	pols := []policiesv1beta1.MutatingPolicyLike{mpol}
	provider, err := NewProvider(compiler.NewCompiler(), pols, nil, libs.NewFakeContextProvider())
	assert.NoError(t, err)

	nsResolverDefault := func(ns string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	}

	// Simulate evaluating a ConfigMap target — this is the resolved target, NOT a secret.
	target := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "test-cm", "namespace": "default"},
	}}
	attr := admission.NewAttributesRecord(
		target, nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"default", "test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"", admission.Update, nil, false, &user.DefaultInfo{},
	)

	eng := NewEngine(provider, nsResolverDefault, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})
	resp, err := eng.Evaluate(ctx, attr, admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Name:      "test-cm",
		Namespace: "default",
	}, predicate)

	assert.NoError(t, err)
	// The policy must NOT be filtered out — expression-only targets should
	// bypass the matcher's constraint check.
	if assert.Len(t, resp.Policies, 1) {
		assert.NotEmpty(t, resp.Policies[0].Rules,
			"expression-only target should not be filtered by trigger matchConstraints")
	}
}

// TestEvaluate_ExpressionOnlyTargetNamespaceSelectorFilters verifies that even
// for expression-only targetMatchConstraints (no resourceRules), a
// NamespaceSelector on the targetMatchConstraints is still evaluated by the
// matcher. A target whose namespace does NOT carry the required label must be
// filtered out (empty Rules).
func TestEvaluate_ExpressionOnlyTargetNamespaceSelectorFilters(t *testing.T) {
	mutateExisting := true

	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "expression-ns-selector"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &mutateExisting,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"secrets"},
						},
					},
				}},
			},
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "production"},
					},
				},
				Expression: `resource.get("v1", "configmaps", object.metadata.namespace, "test-cm")`,
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{metadata: Object.metadata{labels: {"patched": "yes"}}}`,
				},
			}},
		},
	}

	pols := []policiesv1beta1.MutatingPolicyLike{mpol}
	provider, err := NewProvider(compiler.NewCompiler(), pols, nil, libs.NewFakeContextProvider())
	assert.NoError(t, err)

	nsResolverNoLabel := func(ns string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	}

	target := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "test-cm", "namespace": "default"},
	}}
	attr := admission.NewAttributesRecord(
		target, nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"default", "test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"", admission.Update, nil, false, &user.DefaultInfo{},
	)

	eng := NewEngine(provider, nsResolverNoLabel, matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{})
	resp, err := eng.Evaluate(ctx, attr, admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Name:      "test-cm",
		Namespace: "default",
	}, predicate)

	assert.NoError(t, err)
	if assert.Len(t, resp.Policies, 1) {
		assert.Empty(t, resp.Policies[0].Rules,
			"expression-only target with non-matching NamespaceSelector should be filtered out")
	}
}
