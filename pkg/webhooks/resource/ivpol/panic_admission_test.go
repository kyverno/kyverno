package ivpol

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
)

type panickingCompiler struct {
	panicMsg string
}

func (c panickingCompiler) Compile(policiesv1beta1.ImageValidatingPolicyLike, []*policiesv1beta1.PolicyException) (evaluator.CompiledPolicy, field.ErrorList) {
	return panickingCompiledPolicy{panicMsg: c.panicMsg}, nil
}

type panickingCompiledPolicy struct {
	panicMsg string
}

func (p panickingCompiledPolicy) Evaluate(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, runtime.Object, bool, func() (map[string]any, error), libs.Context) (*evaluator.EvaluationResult, error) {
	panic(p.panicMsg)
}

func (p panickingCompiledPolicy) EnforceRequired([]string, *imageverify.ImageVerificationResults) error {
	return nil
}

// Tracing reports false: the stub is never compiled for decision tracing.
func (p panickingCompiledPolicy) Tracing() bool { return false }

func (p panickingCompiledPolicy) MutateDigest(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, runtime.Object, unstructured.Unstructured, func() (map[string]any, error), config.Configuration, libs.Context) ([]jsonpatch.JsonPatchOperation, error) {
	return nil, nil
}

// TestPanicInPolicyDeniesAdmissionWithoutCrashing runs the real webhook handler
// with an engine whose policy evaluation raises a panic. The panic is recovered
// by evaluatePolicies, denying admission without crashing the process.
func TestPanicInPolicyDeniesAdmissionWithoutCrashing(t *testing.T) {
	const image = "registry.example/app:v1"
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-signed-images"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false)},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"},
						},
					},
				}},
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "registry.example/*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	provider, err := ivpolengine.NewProvider(panickingCompiler{panicMsg: "injected panic while evaluating policy"}, []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	namespaceResolver := func(s string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: s}}
	}
	engine := ivpolengine.NewEngine(provider, namespaceResolver, matching.NewMatcher(), nil, imageverifycache.DisabledImageVerifyCache(), config.NewDefaultConfiguration(false))
	h := New(engine, libs.NewFakeContextProvider(), nil, false, event.NewFake())
	request := admissionv1.AdmissionRequest{
		UID: "panic-test", Name: "web", Namespace: "default", Operation: admissionv1.Create,
		Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
		Resource: metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
		Object: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Pod",
			"metadata":{"name":"web","namespace":"default"},
			"spec":{"containers":[{"name":"app","image":"` + image + `"}]}}`)},
	}
	request.RequestResource = &request.Resource

	response := h.ValidateClustered(context.Background(), logr.Discard(), handlers.AdmissionRequest{AdmissionRequest: request}, "", time.Now())
	require.False(t, response.Allowed)
	require.NotNil(t, response.Result)
	require.Contains(t, response.Result.Message, "injected panic while evaluating policy")
}
