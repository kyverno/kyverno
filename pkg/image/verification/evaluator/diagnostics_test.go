package evaluator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

// An invalid public key exercises the real Cosign wrapper without network I/O.
func diagnosticPolicy() *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
		ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false), Required: ptr.To(false)},
		Attestors:                []policiesv1beta1.Attestor{{Name: "bad", Cosign: &policiesv1beta1.Cosign{Key: &policiesv1beta1.Key{Data: "invalid public key"}, CTLog: &policiesv1beta1.CTLog{InsecureIgnoreTlog: true}}}},
	}}
}

func TestEvaluationVerificationDiagnostics(t *testing.T) {
	t.Parallel()
	const verify = `verifyImageSignatures("example.com/image:tag", [attestors.bad])`
	for _, tc := range []struct {
		name, expression, message, messageExpression, want string
		preceding                                          string
		diagnostic                                         bool
	}{
		{name: "configured", expression: verify + ">0", message: "configured", want: "configured", diagnostic: true},
		{name: "dynamic", expression: verify + ">0", message: "configured", messageExpression: `"dynamic"`, want: "dynamic", diagnostic: true},
		{name: "default", expression: verify + ">0", want: "CEL expression validation failed at index 0", diagnostic: true},
		{name: "empty dynamic", expression: verify + ">0", message: "configured", messageExpression: `""`, want: "CEL expression validation failed at index 0", diagnostic: true},
		{name: "broken dynamic", expression: verify + ">0", messageExpression: `string(1 / 0)`, want: "failed to evaluate message expression:", diagnostic: true},
		{name: "previous validation", preceding: verify + ">=0", expression: "false", message: "unrelated", want: "unrelated"},
		{name: "message-only verification", expression: "false", messageExpression: verify + ` == 0 ? "dynamic" : "other"`, want: "dynamic"},
		{name: "repeated verification", preceding: verify + ">=0", expression: verify + ">0", message: "second", want: "second", diagnostic: true},
		{name: "short circuit", expression: "false && " + verify + ">0", message: "unrelated", want: "unrelated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := diagnosticPolicy()
			if tc.preceding != "" {
				p.Spec.Validations = append(p.Spec.Validations, admissionregistrationv1.Validation{Expression: tc.preceding})
			}
			p.Spec.Validations = append(p.Spec.Validations, admissionregistrationv1.Validation{Expression: tc.expression, Message: tc.message, MessageExpression: tc.messageExpression})
			compiled, errs := NewCompiler(nil).Compile(p, nil)
			require.Empty(t, errs)
			req, attr, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
			result, err := compiled.Evaluate(context.Background(), &imageverify.Runtime{ImageContext: mutationImages{}}, attr, req, nil, true, nil, nil)
			require.NoError(t, err)
			require.False(t, result.Result)
			require.Nil(t, result.Error)
			if tc.diagnostic {
				require.True(t, strings.HasPrefix(result.Message, tc.want), result.Message)
				require.Contains(t, result.Message, `; verification details: image "example.com/image:tag", attestor "bad": failed to build cosign verification opts:`)
			} else {
				require.Equal(t, tc.want, result.Message)
			}
		})
	}
}

func TestEvaluationVerificationDiagnosticsCachedVariable(t *testing.T) {
	t.Parallel()
	p := diagnosticPolicy()
	p.Spec.Variables = []admissionregistrationv1.Variable{{Name: "count", Expression: `verifyImageSignatures("example.com/image:tag", [attestors.bad])`}}
	p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "variables.count >= 0"}, {Expression: "variables.count > 0", Message: "cached failure"}}
	compiled, errs := NewCompiler(nil).Compile(p, nil)
	require.Empty(t, errs)
	req, attr, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	result, err := compiled.Evaluate(context.Background(), &imageverify.Runtime{ImageContext: mutationImages{}}, attr, req, nil, true, nil, nil)
	require.NoError(t, err)
	require.False(t, result.Result)
	require.Equal(t, "cached failure", result.Message, "cached variables must not be re-evaluated to regenerate diagnostics")
}

func TestEvaluationVerificationDiagnosticsIsolation(t *testing.T) {
	t.Parallel()
	p := diagnosticPolicy()
	p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: `object.metadata.name == "skip" ? false : verifyImageSignatures("example.com/image:tag", [attestors.bad]) > 0`, Message: "denied"}}
	compiled, errs := NewCompiler(nil).Compile(p, nil)
	require.Empty(t, errs)
	other := diagnosticPolicy()
	other.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "false", Message: "unrelated"}}
	second, errs := NewCompiler(nil).Compile(other, nil)
	require.Empty(t, errs)
	for i := range 32 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			rt := &imageverify.Runtime{ImageContext: mutationImages{}, Results: imageverify.NewImageVerificationResults()}
			req, attr, pod := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
			if i%2 == 0 {
				pod.SetName("skip")
			}
			for range 2 {
				result, err := compiled.Evaluate(context.Background(), rt, attr, req, nil, true, nil, nil)
				require.NoError(t, err)
				require.Equal(t, i%2 != 0, strings.Contains(result.Message, "verification details"))
				result, err = second.Evaluate(context.Background(), rt, attr, req, nil, true, nil, nil)
				require.NoError(t, err)
				require.Equal(t, "unrelated", result.Message)
			}
		})
	}
}
