package mpol

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	configmocks "github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func defaultTestConfig() config.Configuration {
	return config.NewDefaultConfiguration(false)
}

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
func (f *fakeContext) SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache, useServerSideApply bool) {
	panic("not implemented")
}

var (
	kyvernoClient = versioned.Clientset{}
	client        = dclient.NewEmptyFakeClient()
	eng           = mpolengine.NewEngine(nil, nil, nil, nil, nil)
	mapper        = meta.NewDefaultRESTMapper([]schema.GroupVersion{{
		Group:   "kyverno.io",
		Version: "v1",
	}})
	ctx           = &fakeContext{}
	statusControl = common.NewStatusControl(&kyvernoClient, nil)
	reportsConfig = reportutils.NewReportingConfig([]string{})
)

type fakeStatusControl struct {
	failedCalled  bool
	successCalled bool
}

func (f *fakeStatusControl) Failed(name, msg string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.failedCalled = true
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
	return mpolengine.Policy{}, nil
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
		event.NewFake(), defaultTestConfig())

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
		defaultTestConfig(),
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
		defaultTestConfig(),
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

	result := collectGVK(dclient.NewEmptyFakeClient(), mapper, m, "")

	assert.Contains(t, result, "*")
	assert.Equal(t, 1, len(result["*"]))
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
		defaultTestConfig(),
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
		defaultTestConfig(),
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
		defaultTestConfig(),
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

func TestProcess_SkipsFilteredTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := configmocks.NewMockConfiguration(ctrl)
	kubeSystemGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	defaultGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}

	mockConfig.EXPECT().
		ToFilter(kubeSystemGVK, "", "", "kube-system").
		Return(true).
		Times(1)
	mockConfig.EXPECT().
		ToFilter(defaultGVK, "", "", "default").
		Return(false).
		Times(1)

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(kubeSystemGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "NamespaceList"}, &unstructured.UnstructuredList{})

	kubeSystem := &unstructured.Unstructured{}
	kubeSystem.SetGroupVersionKind(kubeSystemGVK)
	kubeSystem.SetName("kube-system")

	defaultNS := &unstructured.Unstructured{}
	defaultNS.SetGroupVersionKind(defaultGVK)
	defaultNS.SetName("default")

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, kubeSystem, defaultNS)
	require.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	kyvernoClient := fake.NewSimpleClientset(&policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "filter-pol"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
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
	})

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	restMapper.Add(kubeSystemGVK, meta.RESTScopeRoot)

	engine := &fakeEngine{}
	engine.On("Evaluate").Return(mpolengine.EngineResponse{}, nil).Once()

	p := NewProcessor(
		fakeClient,
		kyvernoClient,
		engine,
		restMapper,
		&libs.FakeContextProvider{},
		&fakeStatusControl{},
		event.NewFake(),
		mockConfig,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-filter", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "filter-pol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
	}

	err = p.Process(ur)
	assert.NoError(t, err)
	engine.AssertNumberOfCalls(t, "Evaluate", 1)
}

func TestProcess_MutatesUnfilteredTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := configmocks.NewMockConfiguration(ctrl)
	cmGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	mockConfig.EXPECT().
		ToFilter(cmGVK, "", "default", "target-cm").
		Return(false).
		Times(1)

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(cmGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetNamespace("default")
	cm.SetName("target-cm")
	cm.SetUID("uid-1")
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, cm)
	require.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	kyvernoClient := fake.NewSimpleClientset(&policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-pol"},
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
	})

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	restMapper.Add(cmGVK, meta.RESTScopeNamespace)

	patched := cm.DeepCopy()
	patched.SetLabels(map[string]string{"foo": "bar"})

	engine := &fakeEngine{}
	engine.On("Evaluate").Return(mpolengine.EngineResponse{PatchedResource: patched}, nil).Once()

	p := NewProcessor(
		fakeClient,
		kyvernoClient,
		engine,
		restMapper,
		&libs.FakeContextProvider{},
		&fakeStatusControl{},
		event.NewFake(),
		mockConfig,
	)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-mutate", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "mutate-pol",
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
	}

	err = p.Process(ur)
	assert.NoError(t, err)
	engine.AssertNumberOfCalls(t, "Evaluate", 1)
}
