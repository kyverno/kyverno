package mpol

import (
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"

	"context"
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"

	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"

	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

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
	reportsConfig = reportutils.NewReportingConfig()
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

func (f *fakeEngine) Evaluate(ctx context.Context, adm admission.Attributes, amdv1 admissionv1.AdmissionRequest, filter func(policiesv1alpha1.MutatingPolicy) bool) (mpolengine.EngineResponse, error) {
	args := f.Called()
	return args.Get(0).(mpolengine.EngineResponse), args.Error(1)
}

func (f *fakeEngine) Handle(ctx context.Context, engine engine.EngineRequest, filter func(policiesv1alpha1.MutatingPolicy) bool) (mpolengine.EngineResponse, error) {
	args := f.Called()
	return args.Get(0).(mpolengine.EngineResponse), args.Error(1)
}

func (f *fakeEngine) MatchedMutateExistingPolicies(ctx context.Context, engine engine.EngineRequest) []string {
	args := f.Called()
	return args.Get(0).([]string)
}

func TestProcess_NoPolicyFound(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		&fakeEngine{},
		meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "kyverno.io", Version: "v1"}}),
		&fakeContext{},
		reportutils.NewReportingConfig(),
		&fakeStatusControl{},
	)

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
		&policiesv1alpha1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mypol",
			},
			Spec: policiesv1alpha1.MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{{
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

	p := NewProcessor(
		dclient.NewEmptyFakeClient(),
		kyvernoClient,
		engine,
		meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}}),
		&fakeContext{},
		reportutils.NewReportingConfig(),
		&fakeStatusControl{},
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

	result := collectGVK(dclient.NewEmptyFakeClient(), mapper, m)

	assert.Contains(t, result, "*")
	assert.Equal(t, 1, len(result["*"]))
}
