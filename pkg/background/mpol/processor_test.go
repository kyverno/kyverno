package mpol

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

var (
	kyvernoClient = versioned.Clientset{}
	client        = dclient.NewEmptyFakeClient()
	eng           = mpolengine.NewEngine(nil, nil, nil, nil, nil)
	mapper        = meta.NewDefaultRESTMapper([]schema.GroupVersion{{
		Group:   "kyverno.io",
		Version: "v1",
	}})
	statusControl = common.NewStatusControl(&kyvernoClient, nil)
	reportsConfig = reportutils.NewReportingConfig([]string{})
)

type fakeStatusControl struct {
	failedCalled  bool
	failedMsg     string
	successCalled bool
}

func (f *fakeStatusControl) Failed(name, msg string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.failedCalled = true
	f.failedMsg = msg
	return &kyvernov2.UpdateRequest{}, nil
}
func (f *fakeStatusControl) Success(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.successCalled = true
	return &kyvernov2.UpdateRequest{}, nil
}
func (f *fakeStatusControl) Skip(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.successCalled = true
	return &kyvernov2.UpdateRequest{}, nil
}

type fakeEngine struct {
	mock.Mock
}

func (f *fakeEngine) Evaluate(ctx context.Context, adm admission.Attributes, amdv1 admissionv1.AdmissionRequest, filter mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	args := f.Called()
	return args.Get(0).(mpolengine.EngineResponse), args.Error(1)
}

func (f *fakeEngine) GetCompiledPolicy(policyName string) (mpolengine.Policy, error) {
	return mpolengine.Policy{CompiledPolicy: &mpolcompiler.Policy{}}, nil
}

func (f *fakeEngine) GetCompiledPolicies(names ...string) map[string]mpolengine.Policy {
	return map[string]mpolengine.Policy{}
}

func (f *fakeEngine) Handle(ctx context.Context, engine engine.EngineRequest, filter mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	args := f.Called()
	return args.Get(0).(mpolengine.EngineResponse), args.Error(1)
}

func (f *fakeEngine) MatchedMutateExistingPolicies(ctx context.Context, engine engine.EngineRequest) []string {
	args := f.Called()
	return args.Get(0).([]string)
}

func TestProcess_NoPolicyFound(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()
	_ = reportutils.NewReportingConfig([]string{})

	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		&fakeEngine{},
		meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "kyverno.io", Version: "v1"}}),
		&libs.FakeContextProvider{},
		&fakeStatusControl{},
		event.NewFake(),
		nil)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ur1",
			Namespace: "default",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "not-exist",
		},
	}

	err := p.Process(ur)

	assert.NoError(t, err)
}

func TestProcess_EngineEvaluateError(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset(
		&policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mypol",
			},
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
							Rule: admissionregistrationv1alpha1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"ConfigMap"},
							},
						},
					}},
				},
			},
		},
	)

	engine := &fakeEngine{}
	engine.On("Evaluate").Return(mpolengine.EngineResponse{}, errors.New("eval failed"))
	_ = reportutils.NewReportingConfig([]string{})

	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		engine,
		meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}}),
		&libs.FakeContextProvider{},
		&fakeStatusControl{},
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur2", Namespace: "default"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
	}

	err := p.Process(ur)

	assert.NoError(t, err)
}

func TestProcess_FilteredTargetSkipsEngine(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetNamespace("tenant-a")
	cm.SetName("target-cm")
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, cm)
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	kyvernoClient := fake.NewSimpleClientset(
		&policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "bgpol"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
							Rule: admissionregistrationv1alpha1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"configmaps"},
							},
						},
					}},
				},
			},
		},
	)

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"resourceFilters": "[ConfigMap,tenant-a,*]",
		},
	})
	eng := &fakeEngine{}
	sc := &fakeStatusControl{}
	p := NewProcessor(
		fakeClient,
		kyvernoClient,
		eng,
		restMapper,
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		cfg,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-filtered-target", Namespace: "default"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "bgpol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
	}

	assert.NoError(t, p.Process(ur))
	eng.AssertNotCalled(t, "Evaluate")
	assert.True(t, sc.successCalled)
	assert.False(t, sc.failedCalled)
}

func TestProcess_NilAdmissionRequest_DoesNotPanic(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetNamespace("default")
	cm.SetName("target-cm")
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, cm)
	assert.NoError(t, err)
	// Wire up the discovery client (pre-registers configmaps) so ListResource can resolve the GVR.
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	kyvernoClient := fake.NewSimpleClientset(
		&policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "bgpol"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
							Rule: admissionregistrationv1alpha1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"configmaps"},
							},
						},
					}},
				},
			},
		},
	)

	// Register the ConfigMap GVK in the REST mapper so collectGVK can resolve it.
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	eng := &fakeEngine{}
	eng.On("Evaluate").Return(mpolengine.EngineResponse{}, nil)

	p := NewProcessor(
		fakeClient,
		kyvernoClient,
		eng,
		restMapper,
		&libs.FakeContextProvider{},
		&fakeStatusControl{},
		event.NewFake(),
		nil,
	)

	// UR has no AdmissionRequest — this is the background-scan case.
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-bg", Namespace: "default"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "bgpol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: nil, // explicitly nil — background scan
				},
			},
		},
	}

	// Must not panic with nil admission request.
	assert.NotPanics(t, func() {
		_ = p.Process(ur)
	})
}

func TestCollectGVK_NoNamespaceSelector(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	m := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"ConfigMap"},
				},
			},
		}},
	}

	result, err := collectGVK(dclient.NewEmptyFakeClient(), mapper, m, "")

	assert.NoError(t, err)
	assert.Contains(t, result, "*")
	assert.Equal(t, 1, len(result["*"]))
}

func TestCollectGVK_PreservesSubresourceRoute(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)

	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods/resize"},
				},
			},
		}},
	}

	result, err := collectGVK(dclient.NewEmptyFakeClient(), mapper, constraints, "")

	assert.NoError(t, err)
	assert.Len(t, result["*"], 1)
	for item := range result["*"] {
		assert.Equal(t, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, item.parentResource)
		assert.Equal(t, "resize", item.subresource)
	}
}

// TestCollectGVK_RejectsTraversalSubresource covers GHSA-4pqm-wqq6-h9q2: a
// NamespacedMutatingPolicy author controls targetMatchConstraints'
// resources[], and the subresource remainder used to go straight into
// UpdateResource unvalidated. The entry must be dropped with a surfaced
// error instead of silently reaching the dclient chokepoint.
func TestCollectGVK_RejectsTraversalSubresource(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)

	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods/../../../namespaces/kube-system/pods/victim"},
				},
			},
		}},
	}

	result, err := collectGVK(dclient.NewEmptyFakeClient(), mapper, constraints, "")

	assert.Error(t, err)
	assert.Empty(t, result["*"], "a traversing resources[] entry must not resolve to a target route")
}

func TestResolveTargetRoute_Subresource(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
	p := &processor{mapper: mapper}
	pod := unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods/resize"},
				},
			},
		}},
	}

	resource, subresource, scope, err := p.resolveTargetRoute(pod, constraints)

	assert.NoError(t, err)
	assert.Equal(t, meta.RESTScopeNameNamespace, scope)
	assert.Equal(t, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, resource)
	assert.Equal(t, "resize", subresource)
}

// TestResolveTargetRoute_RejectsTraversalSubresource covers GHSA-4pqm-wqq6-h9q2
// at the self-mutation route: a traversing resources[] entry must fail
// loudly instead of handing an unvalidated subresource to UpdateResource.
func TestResolveTargetRoute_RejectsTraversalSubresource(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
	p := &processor{mapper: mapper}
	pod := unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods/../../../namespaces/kube-system/pods/victim"},
				},
			},
		}},
	}

	_, _, _, err := p.resolveTargetRoute(pod, constraints)

	assert.Error(t, err)
}

// TestGetPolicy_NamespacedMutatingPolicy verifies that GetPolicy resolves a
// "namespace/name" UR policy key to a NamespacedMutatingPolicy without hitting
// the cluster-scoped MutatingPolicy API (which rejects "/" in resource names).
func TestGetPolicy_NamespacedMutatingPolicy(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-nmpol",
			Namespace: "my-ns",
		},
	}
	kyvernoClient := fake.NewSimpleClientset(nmpol)
	sc := &fakeStatusControl{}

	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		&fakeEngine{},
		meta.NewDefaultRESTMapper(nil),
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-nmpol", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: "my-ns/my-nmpol"},
	}

	result, err := p.GetPolicy(ur)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "my-nmpol", result.GetName())
	assert.Equal(t, "my-ns", result.GetNamespace())
	// Status control must NOT have been called (no failure).
	assert.False(t, sc.failedCalled)
}

// TestProcess_EmptyPolicyKey verifies that Process fails the UR and returns no error
// when the policy key is empty, rather than silently evaluating with an empty name.
func TestProcess_EmptyPolicyKey(t *testing.T) {
	sc := &fakeStatusControl{}
	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		fake.NewSimpleClientset(),
		&fakeEngine{},
		meta.NewDefaultRESTMapper(nil),
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-empty", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: ""},
	}

	err := p.Process(ur)
	assert.NoError(t, err)
	assert.True(t, sc.failedCalled, "expected UR to be marked failed for empty policy key")
}

// TestGetPolicy_BareNameFallback_NamespacedMutatingPolicy verifies that GetPolicy resolves a
// NamespacedMutatingPolicy when the UR key is a bare name (no namespace/name format).
// Admission-webhook URs store only the bare name because reconciler.MatchesMutateExisting
// returns policy.GetName() instead of namespace/name. The fallback uses AdmissionRequest.Namespace
// to locate the NamespacedMutatingPolicy (valid because NamespacedMutatingPolicies only match
// resources in their own namespace, so the admission namespace equals the policy namespace).
func TestGetPolicy_BareNameFallback_NamespacedMutatingPolicy(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypol",
			Namespace: "test-ns",
		},
	}
	kyvernoClient := fake.NewSimpleClientset(nmpol)
	sc := &fakeStatusControl{}

	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		&fakeEngine{},
		meta.NewDefaultRESTMapper(nil),
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	// UR uses bare name (as created by webhook handler), with AdmissionRequest carrying the namespace.
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-bare", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Namespace: "test-ns",
					},
				},
			},
		},
	}

	result, err := p.GetPolicy(ur)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "mypol", result.GetName())
	assert.Equal(t, "test-ns", result.GetNamespace())
	assert.False(t, sc.failedCalled, "UR should not be marked failed when policy is found via fallback")
}

func TestGetTargetsFromExpression_DeleteUsesOldObject(t *testing.T) {
	oldLibCtx := libs.LibraryContext
	libCtx := libs.NewFakeContextProvider()
	libs.LibraryContext = libCtx
	defer func() {
		libs.LibraryContext = oldLibCtx
	}()

	target := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "trigger-cm",
			"namespace": "default",
		},
	}}
	assert.NoError(t, libCtx.AddResource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, target))

	eng := &fakeEngine{}
	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		fake.NewSimpleClientset(),
		eng,
		meta.NewDefaultRESTMapper(nil),
		&libs.FakeContextProvider{},
		&fakeStatusControl{},
		event.NewFake(),
		nil,
	)

	trigger := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "trigger-cm",
			"namespace": "default",
		},
	}
	raw, err := json.Marshal(trigger)
	assert.NoError(t, err)

	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mypol"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				Expression: "resource.Get(object.apiVersion, object.kind.lowerAscii() + 's', object.metadata.namespace, object.metadata.name)",
			},
		},
	}

	ur := &kyvernov2.UpdateRequest{
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{Raw: raw},
					},
					Operation: admissionv1.Delete,
				},
			},
		},
	}

	targets, err := p.getTargetsFromExpression(context.Background(), ur, mpol)
	assert.NoError(t, err)
	if assert.NotNil(t, targets) {
		if assert.Len(t, targets.Items, 1) {
			assert.Equal(t, "ConfigMap", targets.Items[0].GetKind())
		}
	}
}

func TestProcess_TargetExpressionInvalidURMarksFailed(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mypol"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				Expression: "{'kind': object.kind}",
			},
		},
	}

	sc := &fakeStatusControl{}
	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		fake.NewSimpleClientset(mpol),
		&fakeEngine{},
		meta.NewDefaultRESTMapper(nil),
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-invalid", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
					},
					Operation: admissionv1.Delete,
				},
			},
		},
	}

	err := p.Process(ur)
	assert.NoError(t, err)
	assert.True(t, sc.failedCalled, "expected UR to be marked failed for invalid expression input")
}

// fakeNSClient answers GetResource("v1", "Namespace", ...) so collectGVK's namespace-existence
// lookup succeeds for a namespaced policy, without needing a fully scheme-registered fake dynamic client.
type fakeNSClient struct {
	dclient.Interface
}

func (f *fakeNSClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	if kind == "Namespace" {
		ns := &unstructured.Unstructured{}
		ns.SetAPIVersion("v1")
		ns.SetKind("Namespace")
		ns.SetName(name)
		return ns, nil
	}
	return f.Interface.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
}

// TestCollectGVK_RejectsClusterScopedKindForNamespacedPolicy verifies that a namespaced policy's
// resource rules never resolve a cluster-scoped kind (e.g. Namespace), which would otherwise be
// relisted across the whole cluster instead of being confined to the policy's own namespace.

// TestCollectGVK_RejectsClusterScopedKindForNamespacedPolicy verifies that a namespaced policy's
// resource rules never resolve a cluster-scoped kind (e.g. Namespace), which would otherwise be
// relisted across the whole cluster instead of being confined to the policy's own namespace.
func TestCollectGVK_RejectsClusterScopedKindForNamespacedPolicy(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, meta.RESTScopeRoot)

	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"configmaps", "namespaces"},
				},
			},
		}},
	}

	result, _ := collectGVK(&fakeNSClient{Interface: dclient.NewEmptyFakeClient()}, mapper, constraints, "ns1")

	assert.Contains(t, result, "ns1")
	assert.Len(t, result["ns1"], 1, "only the namespace-scoped ConfigMap kind should be resolved for a namespaced policy")
	for item := range result["ns1"] {
		assert.Equal(t, "ConfigMap", item.gvk.Kind)
	}
}

// TestProcess_RejectsExpressionTargetInAnotherNamespace verifies that a NamespacedMutatingPolicy
// whose targetMatchConstraints.expression resolves a target reporting a different namespace is
// rejected, rather than patched.
//
// The target-expression compiler (both the real one in pkg/cel/policies/mpol/compiler and the
// getResourcesFromExpression fallback used here) requires the expression's static output type to
// be exactly map(string, google.protobuf.Any) - the return type the resource.Get/List/Post CEL
// bindings declare. A hand-written map literal infers as map(string, dyn) and is rejected at
// compile time, so the only expressions that can compile at all return a resource.Get/List/Post
// result verbatim. Those bindings' namespaced implementation (namespacedImpl in the kyverno/sdk
// resource CEL library) always substitutes the policy's own namespace for the query, so this test
// cannot reproduce a live cross-namespace *query*; instead it registers the fake fetch target's
// own self-reported metadata.namespace as "ns2" while still being found by the "ns1"-scoped query,
// modelling a context provider (a different implementation, a bug, or a resource whose storage
// key doesn't match its own reported metadata) that doesn't keep the two in lockstep. The guard
// must not trust the resolved object's self-reported fields without checking them.
func TestProcess_RejectsExpressionTargetInAnotherNamespace(t *testing.T) {
	oldLibCtx := libs.LibraryContext
	libCtx := libs.NewFakeContextProvider()
	libs.LibraryContext = libCtx
	defer func() {
		libs.LibraryContext = oldLibCtx
	}()

	// Register the fake fetch target under "ns1" - the key the namespaced resource.Get binding
	// will query with (it always substitutes the policy's own namespace, "ns1" here) - then mutate
	// its own metadata.namespace to "ns2" afterwards, without changing where it's stored. This
	// models a context provider whose storage key and the object's self-reported namespace can
	// disagree; Process must reject based on the object's own field, not trust the query context.
	target := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "other-ns-cm",
			"namespace": "ns1",
		},
	}}
	assert.NoError(t, libCtx.AddResource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, target))
	target.Object["metadata"].(map[string]any)["namespace"] = "ns2"

	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mypol", Namespace: "ns1"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				Expression: "resource.Get('v1', 'configmaps', 'other-ns-cm')",
			},
		},
	}

	trigger := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "trigger-cm",
			"namespace": "ns1",
		},
	}
	raw, err := json.Marshal(trigger)
	assert.NoError(t, err)

	sc := &fakeStatusControl{}
	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		fake.NewSimpleClientset(nmpol),
		&fakeEngine{},
		m,
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-cross-ns", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "ns1/mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{Raw: raw},
					},
					Operation: admissionv1.Delete,
				},
			},
		},
	}

	err = p.Process(ur)
	assert.NoError(t, err)
	assert.True(t, sc.failedCalled, "expected cross-namespace expression target to be rejected")
	assert.Contains(t, sc.failedMsg, "outside policy namespace")
}

// TestProcess_RejectsExpressionTargetClusterScoped verifies that a NamespacedMutatingPolicy
// whose targetMatchConstraints.expression resolves a cluster-scoped target is rejected. As in
// TestProcess_RejectsExpressionTargetInAnotherNamespace, the expression must return a
// resource.Get/List/Post result verbatim to compile at all, so the fetch target is registered
// under the fake context provider and returned via resource.Get.

// TestProcess_RejectsExpressionTargetClusterScoped verifies that a NamespacedMutatingPolicy
// whose targetMatchConstraints.expression resolves a cluster-scoped target is rejected. As in
// TestProcess_RejectsExpressionTargetInAnotherNamespace, the expression must return a
// resource.Get/List/Post result verbatim to compile at all, so the fetch target is registered
// under the fake context provider and returned via resource.Get.
func TestProcess_RejectsExpressionTargetClusterScoped(t *testing.T) {
	oldLibCtx := libs.LibraryContext
	libCtx := libs.NewFakeContextProvider()
	libs.LibraryContext = libCtx
	defer func() {
		libs.LibraryContext = oldLibCtx
	}()

	target := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name":      "ns2",
			"namespace": "ns1",
		},
	}}
	assert.NoError(t, libCtx.AddResource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}, target))

	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, meta.RESTScopeRoot)

	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mypol", Namespace: "ns1"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				Expression: "resource.Get('v1', 'namespaces', 'ns2')",
			},
		},
	}

	trigger := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "trigger-cm",
			"namespace": "ns1",
		},
	}
	raw, err := json.Marshal(trigger)
	assert.NoError(t, err)

	sc := &fakeStatusControl{}
	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		fake.NewSimpleClientset(nmpol),
		&fakeEngine{},
		m,
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-cluster-scoped", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "ns1/mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{Raw: raw},
					},
					Operation: admissionv1.Delete,
				},
			},
		},
	}

	err = p.Process(ur)
	assert.NoError(t, err)
	assert.True(t, sc.failedCalled, "expected cluster-scoped expression target to be rejected")
	assert.Contains(t, sc.failedMsg, "cluster-scoped")
}

// TestProcess_RejectsMutationThatRedirectsTargetIdentity verifies that Process does not write a
// mutation whose patch changes the resolved target's own namespace/kind/apiVersion/name - the
// confinement checks above only validate the *resolved* target, so the write path must
// independently confirm the mutation didn't redirect itself to a different object before calling
// UpdateResource.

// TestProcess_RejectsMutationThatRedirectsTargetIdentity verifies that Process does not write a
// mutation whose patch changes the resolved target's own namespace/kind/apiVersion/name - the
// confinement checks above only validate the *resolved* target, so the write path must
// independently confirm the mutation didn't redirect itself to a different object before calling
// UpdateResource.
func TestProcess_RejectsMutationThatRedirectsTargetIdentity(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "NamespaceList"}, &unstructured.UnstructuredList{})
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetNamespace("ns1")
	cm.SetName("target-cm")
	// collectGVK looks up the policy's own namespace object to confine the target list -
	// register it so the namespaced policy's target resolution succeeds.
	ns1 := &unstructured.Unstructured{}
	ns1.SetAPIVersion("v1")
	ns1.SetKind("Namespace")
	ns1.SetName("ns1")
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, cm, ns1)
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mypol", Namespace: "ns1"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
						Rule: admissionregistrationv1alpha1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				}},
			},
		},
	}
	kyvernoClient := fake.NewSimpleClientset(nmpol)

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	// The mutation redirects the patched object into a different namespace - this must never
	// be written, regardless of the fact that the originally-resolved target ("ns1/target-cm")
	// was itself perfectly in-bounds.
	redirected := cm.DeepCopy()
	redirected.SetNamespace("ns2")

	engine := &fakeEngine{}
	engine.On("Evaluate").Return(mpolengine.EngineResponse{
		Resource:        cm,
		PatchedResource: redirected,
	}, nil)

	sc := &fakeStatusControl{}
	updateCalled := false
	mockClient := &mockUpdateTrackingClient{
		Interface: fakeClient,
		updateResource: func(ctx context.Context, apiVersion, kind, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
			updateCalled = true
			return nil, nil
		},
	}

	p := NewProcessor(
		mockClient,
		kyvernoClient,
		engine,
		restMapper,
		&libs.FakeContextProvider{},
		sc,
		event.NewFake(),
		nil,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-redirect", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "ns1/mypol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
	}

	err = p.Process(ur)
	assert.NoError(t, err)
	assert.False(t, updateCalled, "UpdateResource must not be called when the patch redirects the target's identity")
	assert.True(t, sc.failedCalled)
	assert.Contains(t, sc.failedMsg, "changed the target's identity")
}

// mockUpdateTrackingClient overrides UpdateResource to observe whether it was called, while
// delegating everything else (including the initial ListResource/GetResource calls) to a real
// fake dclient.Interface.

// mockUpdateTrackingClient overrides UpdateResource to observe whether it was called, while
// delegating everything else (including the initial ListResource/GetResource calls) to a real
// fake dclient.Interface.
type mockUpdateTrackingClient struct {
	dclient.Interface
	updateResource func(ctx context.Context, apiVersion, kind, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error)
}

func (m *mockUpdateTrackingClient) UpdateResource(ctx context.Context, apiVersion, kind, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	if m.updateResource != nil {
		return m.updateResource(ctx, apiVersion, kind, namespace, obj, dryRun, subresources...)
	}
	return m.Interface.UpdateResource(ctx, apiVersion, kind, namespace, obj, dryRun, subresources...)
}
