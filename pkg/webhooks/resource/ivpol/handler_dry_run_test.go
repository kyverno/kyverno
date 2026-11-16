package ivpol

import (
	"context"
	"errors"
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
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gomodules.xyz/jsonpatch/v2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
)

// TestValidate_DryRunEmitsNoEvents: a server-side dry run must not record
// PolicyViolation events. The webhook declares sideEffects: NoneOnDryRun, and
// the request changes nothing. A real request still records them, and a dry
// run still returns the decision the real request would get.
func TestValidate_DryRunEmitsNoEvents(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "live request records a violation", true: "dry run records nothing"}[dryRun], func(t *testing.T) {
			h, events := traceTestHandler(t, false)
			// no team label, so the Audit policy require-team fails
			request := tracePodRequest("web", `{}`)
			if dryRun {
				request.DryRun = ptr.To(true)
			}
			response := h.ValidateClustered(context.Background(), logr.Discard(), request, "", time.Now())
			assert.True(t, response.Allowed)

			if dryRun {
				assert.Empty(t, events.all())
				return
			}
			require.NotEmpty(t, events.all())
			for _, got := range events.all() {
				assert.Equal(t, event.PolicyViolation, got.Reason)
			}
		})
	}
}

// failingDigestCompiler compiles every policy to one whose digest mutation
// fails, so the mutate handler has a policy error to report as an event.
type failingDigestCompiler struct{}

func (failingDigestCompiler) Compile(policiesv1beta1.ImageValidatingPolicyLike, []*policiesv1beta1.PolicyException) (evaluator.CompiledPolicy, field.ErrorList) {
	return failingDigestPolicy{}, nil
}

type failingDigestPolicy struct{}

func (failingDigestPolicy) Evaluate(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, runtime.Object, bool, func() (map[string]any, error), libs.Context) (*evaluator.EvaluationResult, error) {
	return &evaluator.EvaluationResult{}, nil
}

func (failingDigestPolicy) EnforceRequired([]string, *imageverify.ImageVerificationResults) error {
	return nil
}

func (failingDigestPolicy) Tracing() bool { return false }

func (failingDigestPolicy) MutateDigest(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, runtime.Object, unstructured.Unstructured, func() (map[string]any, error), config.Configuration, libs.Context) ([]jsonpatch.JsonPatchOperation, error) {
	return nil, errors.New("digest lookup failed")
}

// TestMutate_DryRunEmitsNoEvents: the digest-pinning phase reports a failed
// mutation as events, but not for a dry run.
func TestMutate_DryRunEmitsNoEvents(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "pin-digests"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			ValidationAction:         []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{MutateDigest: ptr.To(true)},
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
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	provider, err := ivpolengine.NewProvider(failingDigestCompiler{}, []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	namespaceResolver := func(s string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: s}}
	}
	engine := ivpolengine.NewEngine(provider, namespaceResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "live request records the failure", true: "dry run records nothing"}[dryRun], func(t *testing.T) {
			events := &recordedEvents{}
			h := New(engine, libs.NewFakeContextProvider(), nil, false, events)
			request := tracePodRequest("web", `{}`)
			if dryRun {
				request.DryRun = ptr.To(true)
			}
			response := h.MutateClustered(context.Background(), logr.Discard(), request, "", time.Now())
			assert.True(t, response.Allowed)

			if dryRun {
				assert.Empty(t, events.all())
				return
			}
			require.NotEmpty(t, events.all())
			for _, got := range events.all() {
				assert.Equal(t, event.PolicyViolation, got.Reason)
			}
		})
	}
}
