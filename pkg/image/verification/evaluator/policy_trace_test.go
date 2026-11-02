package evaluator

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// verifyBad calls the real cosign verification with an attestor whose key is invalid
// (diagnosticPolicy), so it fails without any network I/O.
const verifyBad = `verifyImageSignatures("example.com/image:tag", [attestors.bad])`

// countingImages is mutationImages that counts image fetches. verifyImageSignatures fetches the
// image before verifying it, so the count tells how many times verification actually ran.
type countingImages struct {
	mutationImages
	gets *atomic.Int32
}

func (c countingImages) Get(ctx context.Context, image string, auth []remote.Option, names []name.Option) (*imagedataloader.ImageData, error) {
	c.gets.Add(1)
	return c.mutationImages.Get(ctx, image, auth, names)
}

func traceTestPolicy(validations ...admissionregistrationv1.Validation) *policiesv1beta1.ImageValidatingPolicy {
	p := diagnosticPolicy()
	p.Name = "trace-test"
	p.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "not-kube-system", Expression: "object.metadata.namespace != 'kube-system'"}}
	p.Spec.Variables = []admissionregistrationv1.Variable{{Name: "podName", Expression: "object.metadata.name"}}
	p.Spec.Validations = validations
	return p
}

// evaluateTraced evaluates p against the nginx Pod of buildRequestMapHoistRequestAndAttr and
// returns the result, the error and how many image fetches the evaluation made.
func evaluateTraced(t *testing.T, traced bool, p *policiesv1beta1.ImageValidatingPolicy, exceptions ...*policiesv1beta1.PolicyException) (*EvaluationResult, error, int32) {
	t.Helper()
	compiled, errs := NewCompilerWithTrace(nil, traced).Compile(p, exceptions)
	require.Empty(t, errs)
	require.Equal(t, traced, compiled.Tracing())
	req, attr, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	gets := &atomic.Int32{}
	images := countingImages{gets: gets}
	result, err := compiled.Evaluate(context.Background(), images, nil, imageverify.NewImageVerificationResults(), attr, req, nil, true, nil, nil)
	return result, err, gets.Load()
}

func TestEvaluate_TracingOff_NoTrace(t *testing.T) {
	result, err, _ := evaluateTraced(t, false, traceTestPolicy(admissionregistrationv1.Validation{Expression: "true"}))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Result)
	assert.Nil(t, result.Trace, "no trace when compiled without tracing")
}

func TestEvaluate_TracingOn_Pass(t *testing.T) {
	result, err, _ := evaluateTraced(t, true, traceTestPolicy(
		admissionregistrationv1.Validation{Expression: "variables.podName == 'nginx'"},
		admissionregistrationv1.Validation{Expression: "object.metadata.namespace == 'default'"},
	))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Result)
	require.NotNil(t, result.Trace)
	d := result.Trace

	assert.Equal(t, trace.VerdictPass, d.Verdict.Status)
	require.Len(t, d.Match, 1)
	assert.Equal(t, "not-kube-system", d.Match[0].Name)
	assert.Equal(t, "true", d.Match[0].Result)
	require.Len(t, d.Variables, 1)
	assert.Equal(t, "podName", d.Variables[0].Name)
	assert.Equal(t, "nginx", d.Variables[0].Result)
	require.Len(t, d.Validations, 2)
	for i, v := range d.Validations {
		assert.Equal(t, i, v.Index)
		assert.Equal(t, trace.VerdictPass, v.Status)
		assert.Equal(t, "true", v.Result)
		assert.NotEmpty(t, v.Nodes, "an expression that verifies nothing gets its full breakdown")
	}
}

// TestEvaluate_TracingOn_VerificationIsNeverRunTwice is option C: an expression that calls image
// verification has no tracking twin, so explaining it never repeats the registry calls (or the
// verification it records). It is still traced, by its source and its result, and the failure
// message keeps the verification details that say why it failed.
func TestEvaluate_TracingOn_VerificationIsNeverRunTwice(t *testing.T) {
	tests := map[string]*policiesv1beta1.ImageValidatingPolicy{
		"in a validation": traceTestPolicy(
			admissionregistrationv1.Validation{Expression: "true"},
			admissionregistrationv1.Validation{Expression: verifyBad + " > 0", Message: "image must be signed"},
		),
		"in a variable the validation reads": func() *policiesv1beta1.ImageValidatingPolicy {
			p := traceTestPolicy(admissionregistrationv1.Validation{Expression: "variables.verified > 0", Message: "image must be signed"})
			p.Spec.Variables = append(p.Spec.Variables, admissionregistrationv1.Variable{Name: "verified", Expression: verifyBad})
			return p
		}(),
	}
	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			untraced, untracedErr, untracedGets := evaluateTraced(t, false, policy)
			traced, tracedErr, tracedGets := evaluateTraced(t, true, policy)
			require.NoError(t, untracedErr)
			require.NoError(t, tracedErr)

			require.Positive(t, untracedGets, "the policy really verifies an image")
			assert.Equal(t, untracedGets, tracedGets, "explaining must not run verification again")
			assert.Equal(t, untraced.Result, traced.Result)
			assert.Equal(t, untraced.Message, traced.Message)
			assert.Contains(t, traced.Message, "verification details", "the message still says why verification failed")

			require.NotNil(t, traced.Trace)
			assert.Equal(t, trace.VerdictFail, traced.Trace.Verdict.Status)
			assert.Equal(t, traced.Message, traced.Trace.Verdict.Message)
			failing := traced.Trace.Validations[len(traced.Trace.Validations)-1]
			assert.Equal(t, trace.VerdictFail, failing.Status)
			assert.Equal(t, "false", failing.Result)
		})
	}
}

func TestEvaluate_TracingOn_VerificationExpressionSaysWhyNoBreakdown(t *testing.T) {
	traced, err, _ := evaluateTraced(t, true, traceTestPolicy(admissionregistrationv1.Validation{Expression: verifyBad + " > 0"}))
	require.NoError(t, err)
	require.NotNil(t, traced.Trace)
	assert.Empty(t, traced.Trace.Verdict.Nodes)
	assert.Equal(t, "it verifies images, and verification is never run a second time", traced.Trace.Verdict.NoBreakdown)

	plain, err, _ := evaluateTraced(t, true, traceTestPolicy(admissionregistrationv1.Validation{Expression: "variables.podName == 'other'"}))
	require.NoError(t, err)
	assert.Empty(t, plain.Trace.Verdict.NoBreakdown, "an expression that verifies nothing has its breakdown")
	assert.NotEmpty(t, plain.Trace.Verdict.Nodes)
}

func TestEvaluate_TracingOn_ImagesShowWhatWasChecked(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "images-trace"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration:  &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false), Required: ptr.To(false)},
			MatchImageReferences:     []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{Name: "workloads", Expression: "[object.app, object.sidecar]"},
				{Name: "init", Expression: "[object.init]"},
			},
			Validations: []admissionregistrationv1.Validation{{Expression: "images.workloads.size() == 1"}},
		},
	}
	payload := map[string]any{"app": "ghcr.io/x/app:1.0", "sidecar": "docker.io/library/busybox:1", "init": "ghcr.io/x/init:2.0"}
	evaluate := func(traced bool) *EvaluationResult {
		compiled, errs := NewCompilerWithTrace(nil, traced).Compile(policy, nil)
		require.Empty(t, errs)
		result, err := compiled.Evaluate(context.Background(), countingImages{gets: &atomic.Int32{}}, nil, imageverify.NewImageVerificationResults(), nil, payload, nil, false, nil, nil)
		require.NoError(t, err)
		return result
	}

	untraced := evaluate(false)
	traced := evaluate(true)
	assert.Equal(t, untraced.Result, traced.Result)
	assert.True(t, traced.Result, "only the ghcr.io workload image is checked")
	assert.Nil(t, untraced.Trace)
	require.NotNil(t, traced.Trace)
	require.NotNil(t, traced.Trace.Images)
	assert.Equal(t, []trace.ImageTrace{
		{Category: "init", Image: "ghcr.io/x/init:2.0", Checked: true},
		{Category: "workloads", Image: "ghcr.io/x/app:1.0", Checked: true},
		{Category: "workloads", Image: "docker.io/library/busybox:1", Checked: false},
	}, traced.Trace.Images.Found, "categories in a stable order, each extractor's own order kept")
}

func TestEvaluate_TracingOn_MatchConditionFalseSkips(t *testing.T) {
	policy := traceTestPolicy(admissionregistrationv1.Validation{Expression: "true"})
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "only-prod", Expression: "object.metadata.namespace == 'prod'"}}

	untraced, err, _ := evaluateTraced(t, false, policy)
	require.NoError(t, err)
	assert.Nil(t, untraced, "tracing off: a skip is still a nil result")

	traced, err, _ := evaluateTraced(t, true, policy)
	require.NoError(t, err)
	require.NotNil(t, traced)
	assert.True(t, traced.Skipped)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictSkip, traced.Trace.Verdict.Status)
	assert.Contains(t, traced.Trace.Verdict.Message, `match condition "only-prod" did not pass`)
	assert.Empty(t, traced.Trace.Variables, "nothing past the match conditions runs")
}

func TestEvaluate_TracingOn_IgnoredMatchErrorNamesTheCondition(t *testing.T) {
	policy := traceTestPolicy(admissionregistrationv1.Validation{Expression: "true"})
	policy.Spec.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{
		{Name: "owner-is-platform", Expression: "object.metadata.labels.owner == 'platform'"},
		{Name: "passes", Expression: "true"},
	}

	traced, err, _ := evaluateTraced(t, true, policy)
	require.NoError(t, err)
	require.NotNil(t, traced)
	assert.True(t, traced.Skipped)
	assert.Contains(t, traced.Trace.Verdict.Message, `match condition "owner-is-platform" failed to evaluate and failurePolicy is Ignore`)
	assert.NotContains(t, traced.Trace.Verdict.Message, `"passes"`)
}

func TestEvaluate_TracingOn_MatchConditionErrorKeepsTrace(t *testing.T) {
	policy := traceTestPolicy(admissionregistrationv1.Validation{Expression: "true"})
	policy.Spec.MatchConditions = append(policy.Spec.MatchConditions, admissionregistrationv1.MatchCondition{
		Name: "owner-is-platform", Expression: "object.metadata.labels.owner == 'platform'",
	})

	untraced, untracedErr, _ := evaluateTraced(t, false, policy)
	require.Error(t, untracedErr)
	assert.Nil(t, untraced, "tracing off: unchanged, a nil result and the error")

	traced, tracedErr, _ := evaluateTraced(t, true, policy)
	require.Error(t, tracedErr)
	assert.Equal(t, untracedErr.Error(), tracedErr.Error())
	require.NotNil(t, traced)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictError, traced.Trace.Verdict.Status)
	require.Len(t, traced.Trace.Match, 2)
	assert.Equal(t, "owner-is-platform", traced.Trace.Match[1].Name)
	assert.Empty(t, traced.Trace.Validations, "no validation ran, so none is listed as not run either")
}

func TestEvaluate_TracingOn_ValidationsAfterAFailureAreNotRun(t *testing.T) {
	traced, err, _ := evaluateTraced(t, true, traceTestPolicy(
		admissionregistrationv1.Validation{Expression: "true"},
		admissionregistrationv1.Validation{Expression: "variables.podName == 'other'", Message: "wrong pod"},
		admissionregistrationv1.Validation{Expression: "true"},
	))
	require.NoError(t, err)
	require.NotNil(t, traced.Trace)
	var statuses []string
	for _, v := range traced.Trace.Validations {
		statuses = append(statuses, v.Status)
	}
	assert.Equal(t, []string{trace.VerdictPass, trace.VerdictFail, trace.VerdictNotRun}, statuses)
	assert.Equal(t, trace.VerdictFail, traced.Trace.Verdict.Status)
	assert.Equal(t, "wrong pod", traced.Trace.Verdict.Message)
}

func TestEvaluate_TracingOn_ExceptionSkips(t *testing.T) {
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "allow-nginx", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "is-nginx", Expression: "object.metadata.name == 'nginx'"}},
		},
	}
	traced, err, gets := evaluateTraced(t, true, traceTestPolicy(admissionregistrationv1.Validation{Expression: verifyBad + " > 0"}), exception)
	require.NoError(t, err)
	require.NotNil(t, traced)
	require.Len(t, traced.Exceptions, 1)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictSkip, traced.Trace.Verdict.Status)
	assert.Equal(t, "rule is skipped due to policy exception: default/allow-nginx", traced.Trace.Verdict.Message)
	assert.Zero(t, gets, "an exempted resource verifies nothing")
}

func TestEvaluate_TracingKeepsTheCostLimit(t *testing.T) {
	const items = "[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]"
	costly := items + ".all(a, " + items + ".all(b, " + items + ".all(c, " + items + ".all(d, " + items + ".all(e, a+b+c+d+e > 0)))))"
	policy := traceTestPolicy(admissionregistrationv1.Validation{Expression: costly})

	_, untracedErr, _ := evaluateTraced(t, false, policy)
	traced, tracedErr, _ := evaluateTraced(t, true, policy)
	require.Error(t, untracedErr, "the cost limit must stop the expression")
	require.Error(t, tracedErr, "tracing must not lift the cost limit")
	assert.Equal(t, untracedErr.Error(), tracedErr.Error())
	assert.Contains(t, tracedErr.Error(), "cost limit exceeded")
	require.NotNil(t, traced)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictError, traced.Trace.Verdict.Status)
}

func TestCallsImageVerification(t *testing.T) {
	compiled, errs := NewCompilerWithTrace(nil, true).Compile(traceTestPolicy(
		admissionregistrationv1.Validation{Expression: verifyBad + " > 0"},
		admissionregistrationv1.Validation{Expression: "object.metadata.name == 'nginx'"},
	), nil)
	require.Empty(t, errs)
	validations := compiled.(*compiledPolicy).validations
	require.Len(t, validations, 2)

	assert.True(t, callsImageVerification(validations[0].AST))
	assert.Nil(t, validations[0].Traced, "an expression that verifies images gets no tracking twin")
	assert.NotNil(t, validations[0].AST, "but keeps its AST, so the trace still shows it")

	assert.False(t, callsImageVerification(validations[1].AST))
	assert.NotNil(t, validations[1].Traced)

	assert.False(t, callsImageVerification(nil))
	assert.Equal(t, engine.TracedProgram{}, withoutVerificationTwin(engine.TracedProgram{}))
}

// jsonTracePolicy is a JSON-mode policy over {"app": <image>}, checking ghcr.io images without
// verifying any signature, so nothing reaches a registry.
func jsonTracePolicy(name string, required bool, validations ...admissionregistrationv1.Validation) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration:  &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false), Required: ptr.To(required)},
			MatchImageReferences:     []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			ImageExtractors:          []policiesv1beta1.ImageExtractor{{Name: "app", Expression: "[object.app]"}},
			Validations:              validations,
		},
	}
}

func TestEvaluateWithTrace_JSONPayload(t *testing.T) {
	payload := map[string]any{"app": "ghcr.io/x/app:1.0"}
	tests := []struct {
		name        string
		policy      *policiesv1beta1.ImageValidatingPolicy
		wantResult  bool
		wantVerdict string
		wantMessage string
	}{{
		name:        "passes",
		policy:      jsonTracePolicy("passes", false, admissionregistrationv1.Validation{Expression: "images.app.size() == 1"}),
		wantResult:  true,
		wantVerdict: trace.VerdictPass,
	}, {
		name:        "fails",
		policy:      jsonTracePolicy("fails", false, admissionregistrationv1.Validation{Expression: "images.app.size() == 0", Message: "no images allowed"}),
		wantVerdict: trace.VerdictFail,
		wantMessage: "no images allowed",
	}, {
		// the validation passes, but nothing verified the image
		name:        "required turns a pass into a failure",
		policy:      jsonTracePolicy("required", true, admissionregistrationv1.Validation{Expression: "true"}),
		wantVerdict: trace.VerdictFail,
		wantMessage: "every validation passed, but validationConfigurations.required failed: image ghcr.io/x/app:1.0 is not verified",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ivpols := []*CompiledImageValidatingPolicy{{Policy: tt.policy}}
			// both runs use a fake image context, so the matched image is never fetched from a
			// registry; Evaluate itself is evaluateWith(false) with a real one
			untraced, err := evaluateWith(context.Background(), countingImages{gets: &atomic.Int32{}}, ivpols, payload, nil, nil, nil, false)
			require.NoError(t, err)
			traced, err := evaluateWith(context.Background(), countingImages{gets: &atomic.Int32{}}, ivpols, payload, nil, nil, nil, true)
			require.NoError(t, err)

			u, tr := untraced[tt.policy.Name], traced[tt.policy.Name]
			require.NotNil(t, u)
			require.NotNil(t, tr)
			assert.Equal(t, tt.wantResult, u.Result)
			assert.Equal(t, u.Result, tr.Result, "tracing must not change the result")
			assert.Equal(t, u.Message, tr.Message)
			assert.Nil(t, u.Trace)

			require.NotNil(t, tr.Trace)
			assert.Equal(t, tt.policy.Name, tr.Trace.PolicyName)
			assert.Equal(t, "ImageValidatingPolicy", tr.Trace.PolicyKind)
			assert.True(t, tr.Trace.Scope.Applied)
			assert.Equal(t, "evaluated against a JSON payload, so no matchConstraints apply", tr.Trace.Scope.Reason)
			assert.Equal(t, tt.wantVerdict, tr.Trace.Verdict.Status)
			if tt.wantMessage != "" {
				assert.Contains(t, tr.Trace.Verdict.Message, tt.wantMessage)
			}
		})
	}
}

func TestEvaluateWithTrace_MatchConditionSkip(t *testing.T) {
	policy := jsonTracePolicy("skipped", false, admissionregistrationv1.Validation{Expression: "false"})
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "is-prod", Expression: "object.?env.orValue('') == 'prod'"}}
	ivpols := []*CompiledImageValidatingPolicy{{Policy: policy}}
	payload := map[string]any{"app": "ghcr.io/x/app:1.0"}

	untraced, err := evaluateWith(context.Background(), countingImages{gets: &atomic.Int32{}}, ivpols, payload, nil, nil, nil, false)
	require.NoError(t, err)
	assert.Nil(t, untraced[policy.Name], "tracing off: a skip is still a nil result")

	traced, err := evaluateWith(context.Background(), countingImages{gets: &atomic.Int32{}}, ivpols, payload, nil, nil, nil, true)
	require.NoError(t, err)
	got := traced[policy.Name]
	require.NotNil(t, got)
	assert.True(t, got.Skipped)
	require.NotNil(t, got.Trace)
	assert.Equal(t, trace.VerdictSkip, got.Trace.Verdict.Status)
	assert.Equal(t, policy.Name, got.Trace.PolicyName)
}

func TestEvaluateWithTrace_ErrorKeepsThePartialTrace(t *testing.T) {
	policy := jsonTracePolicy("errors", false, admissionregistrationv1.Validation{Expression: "object.missing == 'x'"})
	ivpols := []*CompiledImageValidatingPolicy{{Policy: policy}}
	payload := map[string]any{"app": "ghcr.io/x/app:1.0"}

	untraced, untracedErr := evaluateWith(context.Background(), countingImages{gets: &atomic.Int32{}}, ivpols, payload, nil, nil, nil, false)
	require.Error(t, untracedErr)
	assert.Contains(t, untracedErr.Error(), "no such key: missing", "the validation's own error, not a registry one")
	assert.Nil(t, untraced, "tracing off: unchanged, no results and the error")

	traced, tracedErr := evaluateWith(context.Background(), countingImages{gets: &atomic.Int32{}}, ivpols, payload, nil, nil, nil, true)
	require.Error(t, tracedErr)
	assert.Equal(t, untracedErr.Error(), tracedErr.Error(), "the same error is returned")
	got := traced[policy.Name]
	require.NotNil(t, got, "tracing on: the failing policy's partial trace comes back with the error")
	require.NotNil(t, got.Trace)
	assert.Equal(t, trace.VerdictError, got.Trace.Verdict.Status)
	assert.Equal(t, policy.Name, got.Trace.PolicyName)
}
