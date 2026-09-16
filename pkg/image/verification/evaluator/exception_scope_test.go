package evaluator

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// multiImageObj carries two images so a signed-only PolicyException cannot
// legitimately cover the unsigned sibling on the same resource.
func multiImageObj(images ...string) map[string]any {
	return map[string]any{
		"foo": map[string]any{
			"bar": images,
		},
	}
}

func Test_Eval_ImageScopedException_DoesNotExemptOtherImages(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-exception-scope"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: func() *bool { b := false; return &b }(),
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{Glob: "ghcr.io/*"},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{Name: "bar", Expression: "object.foo.bar"},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					// Deny if any extracted image is the known-unsigned fixture.
					Expression: "!('" + unsignedImage + "' in images.bar)",
					Message:    "malicious image detected",
				},
			},
		},
	}

	resource := multiImageObj(signedImage, unsignedImage)

	// Sanity: without an exception the unsigned image is denied.
	baseline, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: policy}}, resource, nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, baseline[policy.Name])
	assert.False(t, baseline[policy.Name].Result)
	assert.Equal(t, "malicious image detected", baseline[policy.Name].Message)
	assert.Empty(t, baseline[policy.Name].Exceptions)

	// PolicyException scoped only to the signed image must NOT fully skip evaluation.
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "signed-only"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: policy.Name, Kind: "ImageValidatingPolicy"},
			},
			Images: []string{signedImage},
		},
	}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{
		{Policy: policy, Exceptions: []*policiesv1beta1.PolicyException{polex}},
	}, resource, nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result[policy.Name])

	if len(result[policy.Name].Exceptions) > 0 {
		t.Fatalf("BUG: PolicyException scoped to images=[%s] caused a full skip of policy evaluation (result.Exceptions=%v), even though the resource also carries an unrelated image (%s) that the exception never listed. ImageValidatingPolicy exceptions ignore the Images/AllowedValues fields entirely and always fully exempt the resource on any MatchConditions match.",
			signedImage, result[policy.Name].Exceptions, unsignedImage)
	}
	assert.False(t, result[policy.Name].Result, "unsigned image must still be denied under a signed-only exception")
	assert.Equal(t, "malicious image detected", result[policy.Name].Message)
}

func Test_Eval_FullException_StillExempts(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-full-exception"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: func() *bool { b := false; return &b }(),
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{Glob: "ghcr.io/*"},
			},
			ImageExtractors: []policiesv1beta1.ImageExtractor{
				{Name: "bar", Expression: "object.foo.bar"},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "false",
					Message:    "always deny",
				},
			},
		},
	}

	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "full"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: policy.Name, Kind: "ImageValidatingPolicy"},
			},
			// No Images / AllowedValues → intentional full exemption.
		},
	}

	result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{
		{Policy: policy, Exceptions: []*policiesv1beta1.PolicyException{polex}},
	}, multiImageObj(unsignedImage), nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result[policy.Name])
	assert.Len(t, result[policy.Name].Exceptions, 1)
}
