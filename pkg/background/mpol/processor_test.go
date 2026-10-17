package mpol

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

	result, err := collectGVK(&fakeNSClient{Interface: dclient.NewEmptyFakeClient()}, mapper, constraints, "ns1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster-scoped")

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

// targetTrackingClient records the namespace every target list and write was issued against,
// so a test can assert on what the processor actually asked the API server for.
type targetTrackingClient struct {
	dclient.Interface
	lists  []string
	writes []string
}

func (c *targetTrackingClient) ListResource(ctx context.Context, apiVersion, kind, namespace string, sel *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	c.lists = append(c.lists, kind+"|"+namespace)
	return c.Interface.ListResource(ctx, apiVersion, kind, namespace, sel)
}

func (c *targetTrackingClient) UpdateResource(ctx context.Context, apiVersion, kind, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	u := unstructured.Unstructured{Object: obj.(map[string]interface{})}
	c.writes = append(c.writes, kind+"|"+namespace+"/"+u.GetName())
	return &u, nil
}

// labellingEngine returns every target with a label added, so Process always has a real patch
// to carry through to UpdateResource.
type labellingEngine struct{ mpolengine.Engine }

func (labellingEngine) Evaluate(ctx context.Context, adm admission.Attributes, _ admissionv1.AdmissionRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	in := adm.GetObject().(*unstructured.Unstructured)
	patched := in.DeepCopy()
	patched.SetLabels(map[string]string{"mutated": "true"})
	return mpolengine.EngineResponse{Resource: in, PatchedResource: patched}, nil
}

func (labellingEngine) GetCompiledPolicy(string) (mpolengine.Policy, error) {
	return mpolengine.Policy{}, nil
}

func unstructuredObj(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	u.SetName(name)
	return u
}

func namespacedTargetPolicy(name, namespace string, groups, resources []string) *policiesv1beta1.NamespacedMutatingPolicy {
	return &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
							Rule: admissionregistrationv1alpha1.Rule{
								APIGroups:   groups,
								APIVersions: []string{"v1"},
								Resources:   resources,
							},
						},
					}},
				},
			},
		},
	}
}

// TestProcess_ResourceRulesConfinesNamespacedPolicyToItsOwnNamespace drives the resource-rules
// target arm end to end for a namespaced policy. The policy's rules name a namespace-scoped CRD
// whose Kind is "Namespace" in a non-core group, alongside a second ordinary kind.
//
// Deciding whether to list cluster-wide by kind *name* rather than by REST *scope* let this CRD
// clear the policy's namespace confinement, and because the namespace was held in the loop's own
// range variable the cleared value then applied to every later kind in the same rule set - so the
// ConfigMap rule listed and patched ConfigMaps in every namespace. Scope is the only oracle that
// answers this question correctly, so both the filter and the list call must use it.
//
// Set iteration order decides which kind is visited first, so this runs several times.

// TestProcess_ResourceRulesConfinesNamespacedPolicyToItsOwnNamespace drives the resource-rules
// target arm end to end for a namespaced policy. The policy's rules name a namespace-scoped CRD
// whose Kind is "Namespace" in a non-core group, alongside a second ordinary kind.
//
// Deciding whether to list cluster-wide by kind *name* rather than by REST *scope* let this CRD
// clear the policy's namespace confinement, and because the namespace was held in the loop's own
// range variable the cleared value then applied to every later kind in the same rule set - so the
// ConfigMap rule listed and patched ConfigMaps in every namespace. Scope is the only oracle that
// answers this question correctly, so both the filter and the list call must use it.
//
// Set iteration order decides which kind is visited first, so this runs several times.
func TestProcess_ResourceRulesConfinesNamespacedPolicyToItsOwnNamespace(t *testing.T) {
	crdGV := schema.GroupVersion{Group: "foo.example.com", Version: "v1"}
	rbacGV := schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}
	scheme := runtime.NewScheme()
	for _, gvk := range []schema.GroupVersionKind{
		{Version: "v1", Kind: "ConfigMap"}, {Version: "v1", Kind: "Namespace"},
		crdGV.WithKind("Namespace"), rbacGV.WithKind("ClusterRole"),
	} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	}

	for i := 0; i < 20; i++ {
		fakeClient, err := dclient.NewFakeClient(scheme,
			map[schema.GroupVersionResource]string{
				{Version: "v1", Resource: "configmaps"}:                                       "ConfigMapList",
				{Version: "v1", Resource: "namespaces"}:                                       "NamespaceList",
				{Group: "foo.example.com", Version: "v1", Resource: "namespaces"}:             "NamespaceList",
				{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
			},
			// collectGVK confirms the policy's namespace exists before resolving any target.
			unstructuredObj("v1", "Namespace", "", "ns1"),
			unstructuredObj("v1", "ConfigMap", "ns1", "mine"),
			unstructuredObj("v1", "ConfigMap", "victim-ns", "victim-cm"),
			unstructuredObj("foo.example.com/v1", "Namespace", "ns1", "crd-instance"),
			unstructuredObj("foo.example.com/v1", "Namespace", "victim-ns", "victim-crd"),
			unstructuredObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "cluster-admin"),
		)
		assert.NoError(t, err)
		// Discovery must resolve the custom Namespace separately from the core Namespace.
		fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
			crdGV.WithResource("namespaces"),
			rbacGV.WithResource("clusterroles"),
		}))
		tc := &targetTrackingClient{Interface: fakeClient}

		m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}, crdGV, rbacGV})
		m.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
		// core v1 Namespace is deliberately absent from the mapper: with it, KindFor for
		// "/v1 namespaces" is ambiguous against the CRD below and resolves to neither.
		// genuinely namespace-scoped, so a REST-scope filter keeps it despite the name
		m.Add(crdGV.WithKind("Namespace"), meta.RESTScopeNamespace)
		// genuinely cluster-scoped, so it must be dropped for a namespaced policy
		m.Add(rbacGV.WithKind("ClusterRole"), meta.RESTScopeRoot)

		nmpol := namespacedTargetPolicy("confined", "ns1",
			[]string{"foo.example.com", "", "rbac.authorization.k8s.io"},
			[]string{"namespaces", "configmaps", "clusterroles"})

		p := NewProcessor(tc, fake.NewSimpleClientset(nmpol), labellingEngine{}, m,
			&libs.FakeContextProvider{}, &fakeStatusControl{}, event.NewFake(), nil)

		assert.NoError(t, p.Process(&kyvernov2.UpdateRequest{
			ObjectMeta: metav1.ObjectMeta{Name: "ur-confined", Namespace: "kyverno"},
			Spec:       kyvernov2.UpdateRequestSpec{Policy: "ns1/confined"},
		}))

		// Without these the assertions below hold vacuously: an empty client or a failed
		// namespace lookup resolves nothing, and every "all writes are in ns1" check passes.
		assert.NotEmpty(t, tc.lists, "test resolved no targets at all, so it proves nothing")
		assert.NotEmpty(t, tc.writes, "test wrote nothing at all, so it proves nothing")
		for _, l := range tc.lists {
			assert.True(t, strings.HasSuffix(l, "|ns1"),
				"every target list must be confined to the policy namespace, got %q (all: %v)", l, tc.lists)
		}
		assert.Contains(t, tc.writes, "ConfigMap|ns1/mine", "the ordinary namespaced target must be mutated")
		assert.Contains(t, tc.writes, "Namespace|ns1/crd-instance", "the namespaced CRD must also be mutated")
		for _, w := range tc.writes {
			assert.Contains(t, []string{"ConfigMap|ns1/mine", "Namespace|ns1/crd-instance"}, w,
				"namespaced policy wrote to an unexpected target: %q (all: %v)", w, tc.writes)
		}
	}
}

// TestProcess_ResourceRulesRecordsFailureForDroppedClusterScopedKind: dropping a cluster-scoped
// kind from a namespaced policy's rules must be reported, not silent. Otherwise a policy whose
// only rule is cluster-scoped resolves no targets and the UpdateRequest is marked Succeeded
// having done nothing, leaving the operator no signal. The expression arm already records a
// failure for the same condition.

// TestProcess_ResourceRulesRecordsFailureForDroppedClusterScopedKind: dropping a cluster-scoped
// kind from a namespaced policy's rules must be reported, not silent. Otherwise a policy whose
// only rule is cluster-scoped resolves no targets and the UpdateRequest is marked Succeeded
// having done nothing, leaving the operator no signal. The expression arm already records a
// failure for the same condition.
func TestProcess_ResourceRulesRecordsFailureForDroppedClusterScopedKind(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "NamespaceList"}, &unstructured.UnstructuredList{})
	fakeClient, err := dclient.NewFakeClient(scheme,
		map[schema.GroupVersionResource]string{{Version: "v1", Resource: "namespaces"}: "NamespaceList"},
		unstructuredObj("v1", "Namespace", "", "ns1"))
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, meta.RESTScopeRoot)

	nmpol := namespacedTargetPolicy("cluster-rule", "ns1", []string{""}, []string{"namespaces"})

	sc := &fakeStatusControl{}
	p := NewProcessor(fakeClient, fake.NewSimpleClientset(nmpol), labellingEngine{}, m,
		&libs.FakeContextProvider{}, sc, event.NewFake(), nil)

	assert.NoError(t, p.Process(&kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-dropped", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: "ns1/cluster-rule"},
	}))

	assert.True(t, sc.failedCalled, "dropping the only target rule must be reported, not silently succeed")
	assert.False(t, sc.successCalled)
	assert.Contains(t, sc.failedMsg, "cluster-scoped")
}

// TestProcess_ClusterScopedPolicyStillTargetsClusterScopedKinds is the regression guard for the
// confinement above: MutatingPolicy and NamespacedMutatingPolicy share one spec type, and a
// cluster-scoped MutatingPolicy mutating every Namespace is its legitimate job.

// TestProcess_ClusterScopedPolicyStillTargetsClusterScopedKinds is the regression guard for the
// confinement above: MutatingPolicy and NamespacedMutatingPolicy share one spec type, and a
// cluster-scoped MutatingPolicy mutating every Namespace is its legitimate job.
func TestProcess_ClusterScopedPolicyStillTargetsClusterScopedKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "NamespaceList"}, &unstructured.UnstructuredList{})
	fakeClient, err := dclient.NewFakeClient(scheme,
		map[schema.GroupVersionResource]string{{Version: "v1", Resource: "namespaces"}: "NamespaceList"},
		unstructuredObj("v1", "Namespace", "", "ns1"), unstructuredObj("v1", "Namespace", "", "ns2"))
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	tc := &targetTrackingClient{Interface: fakeClient}

	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, meta.RESTScopeRoot)

	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "psa-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
							Rule: admissionregistrationv1alpha1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"namespaces"},
							},
						},
					}},
				},
			},
		},
	}

	p := NewProcessor(tc, fake.NewSimpleClientset(mpol), labellingEngine{}, m,
		&libs.FakeContextProvider{}, &fakeStatusControl{}, event.NewFake(), nil)

	assert.NoError(t, p.Process(&kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-cluster", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: "psa-label"},
	}))

	assert.Equal(t, []string{"Namespace|"}, tc.lists, "cluster-scoped policy must list cluster-wide")
	assert.Len(t, tc.writes, 2, "cluster-scoped policy must still mutate every Namespace")
}

// TestCollectGVK_FailsClosedWhenRESTScopeIsUnknown: a kind the mapper can name but cannot map
// has an unknown REST scope, and an unknown scope must never be treated as safe to list
// cluster-wide. It is dropped, and reported rather than dropped silently.

// TestCollectGVK_FailsClosedWhenRESTScopeIsUnknown: a kind the mapper can name but cannot map
// has an unknown REST scope, and an unknown scope must never be treated as safe to list
// cluster-wide. It is dropped, and reported rather than dropped silently.
func TestCollectGVK_FailsClosedWhenRESTScopeIsUnknown(t *testing.T) {
	// unmappableMapper resolves the kind but refuses to map it, which is what a partially
	// discovered CRD looks like to the processor.
	mapper := &unmappableMapper{RESTMapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})}
	mapper.RESTMapper.(*meta.DefaultRESTMapper).Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
				Rule: admissionregistrationv1alpha1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"configmaps"},
				},
			},
		}},
	}

	for _, policyNS := range []string{"ns1", ""} {
		result, skipped := collectGVK(&fakeNSClient{Interface: dclient.NewEmptyFakeClient()}, mapper, constraints, policyNS)
		for _, set := range result {
			assert.Equal(t, 0, set.Len(), "a kind with no REST mapping must not be resolved (policy namespace %q)", policyNS)
		}
		assert.Error(t, skipped, "dropping a kind with an unknown REST scope must be reported")
		assert.Contains(t, skipped.Error(), "REST scope")
	}
}

type unmappableMapper struct {
	meta.RESTMapper
}

func (unmappableMapper) RESTMapping(schema.GroupKind, ...string) (*meta.RESTMapping, error) {
	return nil, errors.New("no matches for kind")
}

// TestCollectGVK_ReportsWhenNoKindResolves: resource rules are a group x version x resource
// cross-product, so individual KindFor misses are expected and not worth reporting. But when
// the whole rule set resolves nothing - a misspelled resource, a CRD that was never discovered -
// the policy would otherwise resolve no targets and the UpdateRequest would report Succeeded
// having done nothing.
func TestCollectGVK_ReportsWhenNoKindResolves(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)

	constraints := admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
				Rule: admissionregistrationv1alpha1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"widgets"}, // no such resource
				},
			},
		}},
	}

	_, skipped := collectGVK(&fakeNSClient{Interface: dclient.NewEmptyFakeClient()}, mapper, constraints, "ns1")
	assert.Error(t, skipped, "a rule set that resolves no kind at all must be reported")
	assert.Contains(t, skipped.Error(), "no target kinds could be resolved")
}

// TestProcess_ClusterScopedKindListedOnceUnderNamespaceSelector: a cluster-scoped kind is the
// same set of objects whichever selected namespace led to it, so a cluster policy with a
// namespaceSelector must list it once, not once per matched namespace. Otherwise every target
// is resolved and written once per namespace the selector matched.

// TestProcess_ClusterScopedKindListedOnceUnderNamespaceSelector: a cluster-scoped kind is the
// same set of objects whichever selected namespace led to it, so a cluster policy with a
// namespaceSelector must list it once, not once per matched namespace. Otherwise every target
// is resolved and written once per namespace the selector matched.
func TestProcess_ClusterScopedKindListedOnceUnderNamespaceSelector(t *testing.T) {
	// A stub rather than the fake dynamic client: this test is about how many times
	// ListResource is called, and the fake discovery client cannot resolve a non-core GVR.
	sc := &scriptedListClient{
		Interface:    dclient.NewEmptyFakeClient(),
		namespaces:   []string{"ns1", "ns2", "ns3"},
		clusterRoles: []string{"cluster-admin"},
	}

	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}, {Group: "rbac.authorization.k8s.io", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, meta.RESTScopeRoot)

	// Cluster-scoped policy whose namespaceSelector matches three namespaces, targeting a
	// cluster-scoped kind.
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-roles"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "app"}},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
							Rule: admissionregistrationv1alpha1.Rule{
								APIGroups:   []string{"rbac.authorization.k8s.io"},
								APIVersions: []string{"v1"},
								Resources:   []string{"clusterroles"},
							},
						},
					}},
				},
			},
		},
	}

	p := NewProcessor(sc, fake.NewSimpleClientset(mpol), labellingEngine{}, m,
		&libs.FakeContextProvider{}, &fakeStatusControl{}, event.NewFake(), nil)

	assert.NoError(t, p.Process(&kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-dedupe", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: "cluster-roles"},
	}))

	assert.Equal(t, 1, sc.clusterRoleLists,
		"a cluster-scoped kind must be listed once, not once per selected namespace")
	assert.Equal(t, []string{"cluster-admin"}, sc.written)
}

// scriptedListClient answers just the calls this dedupe test needs, counting how many times the
// cluster-scoped kind was listed.

// scriptedListClient answers just the calls this dedupe test needs, counting how many times the
// cluster-scoped kind was listed.
type scriptedListClient struct {
	dclient.Interface
	namespaces       []string
	clusterRoles     []string
	clusterRoleLists int
	written          []string
}

func (c *scriptedListClient) ListResource(_ context.Context, apiVersion, kind, namespace string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	out := &unstructured.UnstructuredList{}
	switch kind {
	case "Namespace":
		for _, n := range c.namespaces {
			out.Items = append(out.Items, *unstructuredObj("v1", "Namespace", "", n))
		}
	case "ClusterRole":
		c.clusterRoleLists++
		for _, n := range c.clusterRoles {
			out.Items = append(out.Items, *unstructuredObj(apiVersion, "ClusterRole", "", n))
		}
	}
	return out, nil
}

func (c *scriptedListClient) GetResource(_ context.Context, apiVersion, kind, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	return unstructuredObj(apiVersion, kind, namespace, name), nil
}

func (c *scriptedListClient) UpdateResource(_ context.Context, _, _, _ string, obj interface{}, _ bool, _ ...string) (*unstructured.Unstructured, error) {
	u := unstructured.Unstructured{Object: obj.(map[string]interface{})}
	c.written = append(c.written, u.GetName())
	return &u, nil
}

func labelled(u *unstructured.Unstructured, labels map[string]string) *unstructured.Unstructured {
	u.SetLabels(labels)
	return u
}

// removeKeyEngine mutates whatever object it is given, the way a real policy
// does: it drops data["a"]. onFirstEvaluate lets a test land a write from
// someone else while the policy is being evaluated, and second, when set,
// replaces the result of the second evaluation.
type removeKeyEngine struct {
	fakeEngine
	onFirstEvaluate func()
	second          func(live *unstructured.Unstructured) (mpolengine.EngineResponse, error)
	calls           int
}

func (e *removeKeyEngine) Evaluate(_ context.Context, attr admission.Attributes, _ admissionv1.AdmissionRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	e.calls++
	if e.calls == 1 && e.onFirstEvaluate != nil {
		e.onFirstEvaluate()
	}
	obj := attr.GetObject().(*unstructured.Unstructured)
	if e.calls == 2 && e.second != nil {
		return e.second(obj)
	}
	patched := obj.DeepCopy()
	unstructured.RemoveNestedField(patched.Object, "data", "a")
	return mpolengine.EngineResponse{Resource: obj, PatchedResource: patched}, nil
}

// newRemoveAFixture returns a fake cluster holding the shared/shared-cm ConfigMap (data {"a": "1"})
// and a "remove-a" MutatingPolicy that targets it.
func newRemoveAFixture(t *testing.T) (dclient.Interface, versioned.Interface, meta.RESTMapper) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetNamespace("shared")
	cm.SetName("shared-cm")
	cm.SetResourceVersion("1")
	assert.NoError(t, unstructured.SetNestedStringMap(cm.Object, map[string]string{"a": "1"}, "data"))
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, cm)
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	kyvernoClient := fake.NewSimpleClientset(
		&policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "remove-a"},
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
	return fakeClient, kyvernoClient, restMapper
}

func runRemoveA(t *testing.T, client dclient.Interface, kyvernoClient versioned.Interface, restMapper meta.RESTMapper, eng mpolengine.Engine) (map[string]string, *fakeStatusControl) {
	sc := &fakeStatusControl{}
	p := NewProcessor(client, kyvernoClient, eng, restMapper, &libs.FakeContextProvider{}, sc, event.NewFake(), config.NewDefaultConfiguration(false))
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-remove-a", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Policy: "remove-a"},
	}
	assert.NoError(t, p.Process(ur))
	final, err := client.GetResource(context.Background(), "v1", "ConfigMap", "shared", "shared-cm")
	assert.NoError(t, err)
	data, _, err := unstructured.NestedStringMap(final.Object, "data")
	assert.NoError(t, err)
	return data, sc
}

// addDataB is a write from someone else: it adds data["b"] to the shared ConfigMap.
func addDataB(t *testing.T, client dclient.Interface) {
	ctx := context.Background()
	live, err := client.GetResource(ctx, "v1", "ConfigMap", "shared", "shared-cm")
	assert.NoError(t, err)
	assert.NoError(t, unstructured.SetNestedField(live.Object, "2", "data", "b"))
	// the API server gives every write a new resourceVersion; the fake client does not
	live.SetResourceVersion("2")
	_, err = client.UpdateResource(ctx, "v1", "ConfigMap", "shared", live.Object, false)
	assert.NoError(t, err)
}

func TestProcess_TargetChangedDuringEvaluation_KeepsTheOtherWrite(t *testing.T) {
	fakeClient, kyvernoClient, restMapper := newRemoveAFixture(t)
	// Another writer adds data["b"] after the target was listed but before
	// the policy's change is written back.
	eng := &removeKeyEngine{onFirstEvaluate: func() { addDataB(t, fakeClient) }}
	data, sc := runRemoveA(t, fakeClient, kyvernoClient, restMapper, eng)
	assert.Equal(t, map[string]string{"b": "2"}, data)
	assert.False(t, sc.failedCalled)
}

// When the target changed since it was listed and re-evaluating it does not give a new
// patch, the patch computed from the listed copy must not be written over the live object.
func TestProcess_TargetChangedDuringEvaluation_ReevaluationWithoutPatch(t *testing.T) {
	tests := []struct {
		name       string
		second     func(live *unstructured.Unstructured) (mpolengine.EngineResponse, error)
		wantFailed bool
	}{{
		name: "the re-evaluation errors",
		second: func(*unstructured.Unstructured) (mpolengine.EngineResponse, error) {
			return mpolengine.EngineResponse{}, errors.New("cel evaluation failed")
		},
		wantFailed: true,
	}, {
		name: "the re-evaluation no longer matches",
		second: func(live *unstructured.Unstructured) (mpolengine.EngineResponse, error) {
			return mpolengine.EngineResponse{Resource: live}, nil
		},
	}, {
		name: "the re-evaluation changes nothing",
		second: func(live *unstructured.Unstructured) (mpolengine.EngineResponse, error) {
			return mpolengine.EngineResponse{Resource: live, PatchedResource: live.DeepCopy()}, nil
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, kyvernoClient, restMapper := newRemoveAFixture(t)
			eng := &removeKeyEngine{onFirstEvaluate: func() { addDataB(t, fakeClient) }, second: tt.second}
			data, sc := runRemoveA(t, fakeClient, kyvernoClient, restMapper, eng)
			// writing the stale patch would drop both "a" and the other writer's "b"
			assert.Equal(t, map[string]string{"a": "1", "b": "2"}, data)
			assert.Equal(t, 2, eng.calls)
			assert.Equal(t, tt.wantFailed, sc.failedCalled)
		})
	}
}

func TestProcess_TargetUnchangedSinceList_EvaluatesOnce(t *testing.T) {
	fakeClient, kyvernoClient, restMapper := newRemoveAFixture(t)
	eng := &removeKeyEngine{}
	data, _ := runRemoveA(t, fakeClient, kyvernoClient, restMapper, eng)
	assert.Equal(t, map[string]string{}, data)
	// a second evaluation would record the policy result twice and repeat any http calls
	assert.Equal(t, 1, eng.calls)
}

// notCompiledEngine models the mpol engine before its reconciler has loaded a
// newly created policy: it doesn't know the policy and evaluates nothing.
type notCompiledEngine struct {
	fakeEngine
	compiled  policiesv1beta1.MutatingPolicyLike
	evaluated bool
}

func (e *notCompiledEngine) GetCompiledPolicy(policyName string) (mpolengine.Policy, error) {
	if e.compiled != nil {
		return mpolengine.Policy{Policy: e.compiled}, nil
	}
	return mpolengine.Policy{}, errors.New("policy with name " + policyName + " wasn't found")
}

func (e *notCompiledEngine) Evaluate(_ context.Context, attr admission.Attributes, _ admissionv1.AdmissionRequest, _ mpolengine.Predicate) (mpolengine.EngineResponse, error) {
	e.evaluated = true
	obj, _ := attr.GetObject().(*unstructured.Unstructured)
	return mpolengine.EngineResponse{Resource: obj}, nil
}

// newBackgroundScanFixture mirrors the namespaced-mutating-policies existing/background-scan
// conformance test: an admission-disabled mutate-existing policy and its target ConfigMap.
func newBackgroundScanFixture(t *testing.T, policies ...runtime.Object) (dclient.Interface, versioned.Interface, meta.RESTMapper) {
	scheme := runtime.NewScheme()
	for _, kind := range []string{"ConfigMap", "Namespace"} {
		scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: kind}, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: kind + "List"}, &unstructured.UnstructuredList{})
	}
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName("test-nmpol-background-scan-ns")
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetNamespace("test-nmpol-background-scan-ns")
	cm.SetName("test-nmpol-background-scan-cm")
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, ns, cm)
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	return fakeClient, fake.NewSimpleClientset(policies...), restMapper
}

func backgroundScanSpec() policiesv1beta1.MutatingPolicySpec {
	admissionEnabled, mutateExisting := false, true
	return policiesv1beta1.MutatingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
			Admission:                   &policiesv1beta1.AdmissionConfiguration{Enabled: &admissionEnabled},
			MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: &mutateExisting},
		},
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				ResourceNames: []string{"test-nmpol-background-scan-cm"},
				RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1alpha1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				},
			}},
		},
	}
}

func TestProcess_PolicyNotYetCompiled_RetriesWithoutEvaluating(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol-background-scan", Namespace: "test-nmpol-background-scan-ns"},
		Spec:       backgroundScanSpec(),
	}
	fakeClient, kyvernoClient, restMapper := newBackgroundScanFixture(t, nmpol)
	eng := &notCompiledEngine{}
	sc := &fakeStatusControl{}
	p := NewProcessor(fakeClient, kyvernoClient, eng, restMapper, &libs.FakeContextProvider{}, sc, event.NewFake(), config.NewDefaultConfiguration(false))

	// the update request the policy controller creates when the policy is created
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-policy-event", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELMutate,
			Policy: "test-nmpol-background-scan-ns/test-nmpol-background-scan",
		},
	}
	// an error (rather than a status update) lets the controller retry with backoff
	assert.Error(t, p.Process(ur))
	assert.False(t, sc.successCalled, "a scan that evaluated nothing must not be marked completed")
	assert.False(t, sc.failedCalled, "a failed status is retried at once and deleted after a few tries")
	assert.False(t, eng.evaluated)
}

func TestProcess_SameNamedPolicyFromAnotherScope_DoesNotCountAsCompiled(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol-background-scan"},
		Spec:       backgroundScanSpec(),
	}
	fakeClient, kyvernoClient, restMapper := newBackgroundScanFixture(t, mpol)
	// only the namespaced policy with the same name has been compiled so far
	eng := &notCompiledEngine{compiled: &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol-background-scan", Namespace: "test-nmpol-background-scan-ns"},
		Spec:       backgroundScanSpec(),
	}}
	sc := &fakeStatusControl{}
	p := NewProcessor(fakeClient, kyvernoClient, eng, restMapper, &libs.FakeContextProvider{}, sc, event.NewFake(), config.NewDefaultConfiguration(false))

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-policy-event", Namespace: "kyverno"},
		Spec:       kyvernov2.UpdateRequestSpec{Type: kyvernov2.CELMutate, Policy: "test-nmpol-background-scan"},
	}
	assert.Error(t, p.Process(ur))
	assert.False(t, sc.successCalled)
	assert.False(t, eng.evaluated)
}
