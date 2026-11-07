package apply

import (
	"bytes"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	kyvernoreports "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func Test_policyKindForSource(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{kyvernoreports.SourceValidatingPolicy, "ValidatingPolicy"},
		{kyvernoreports.SourceImageValidatingPolicy, "ImageValidatingPolicy"},
		{kyvernoreports.SourceMutatingPolicy, "MutatingPolicy"},
		{kyvernoreports.SourceGeneratingPolicy, "GeneratingPolicy"},
		{kyvernoreports.SourceValidatingAdmissionPolicy, ""},
		{kyvernoreports.SourceMutatingAdmissionPolicy, ""},
		{"", ""},
		{"unknown-source", ""},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			assert.Equal(t, tt.want, policyKindForSource(tt.source))
		})
	}
}

func compileMatchExpression(t *testing.T, expression string) {
	t.Helper()
	errs := compiler.CompilePolicyExceptionMatchConditions([]admissionregistrationv1.MatchCondition{
		{Name: "match-resource", Expression: expression},
	}, nil)
	assert.Empty(t, errs, "expression %q failed to compile: %v", expression, errs)
}

func Test_buildExceptionMatchExpression(t *testing.T) {
	tests := []struct {
		name      string
		kinds     []string
		names     []string
		namespace string
		want      string
	}{
		{
			name:  "exact name, no namespace",
			kinds: []string{"Pod"},
			names: []string{"my-pod"},
			want:  `object.kind in ["Pod"] && (object.metadata.name == "my-pod")`,
		},
		{
			name:      "exact name with namespace",
			kinds:     []string{"Pod"},
			names:     []string{"my-pod"},
			namespace: "default",
			want:      `object.kind in ["Pod"] && (object.metadata.name == "my-pod") && object.metadata.namespace == "default"`,
		},
		{
			name:  "trailing wildcard name uses startsWith",
			kinds: []string{"CronJob", "Job", "Pod"},
			names: []string{"my-cronjob", "my-cronjob-*"},
			want:  `object.kind in ["CronJob", "Job", "Pod"] && (object.metadata.name == "my-cronjob" || object.metadata.name.startsWith("my-cronjob-"))`,
		},
		{
			name:  "no names",
			kinds: []string{"Pod"},
			names: nil,
			want:  `object.kind in ["Pod"]`,
		},
		{
			name:      "quotes and backslashes in name and namespace",
			kinds:     []string{"Pod"},
			names:     []string{`weird"name\here`},
			namespace: `weird"ns\here`,
			want:      `object.kind in ["Pod"] && (object.metadata.name == "weird\"name\\here") && object.metadata.namespace == "weird\"ns\\here"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExceptionMatchExpression(tt.kinds, tt.names, tt.namespace)
			assert.Equal(t, tt.want, got)
			compileMatchExpression(t, got)
		})
	}
}

func Test_printException(t *testing.T) {
	t.Run("unsupported source produces no output", func(t *testing.T) {
		var buf bytes.Buffer
		result := openreportsv1alpha1.ReportResult{
			Source: kyvernoreports.SourceValidatingAdmissionPolicy,
			Policy: "some-policy",
			Result: "fail",
			Subjects: []corev1.ObjectReference{
				{Kind: "Pod", Name: "my-pod", Namespace: "default"},
			},
		}
		err := printException(&buf, result, 0, "yaml")
		require.NoError(t, err)
		assert.Zero(t, buf.Len())
	})

	t.Run("autogen CronJob rule includes Job and Pod kinds and wildcard name", func(t *testing.T) {
		var buf bytes.Buffer
		result := openreportsv1alpha1.ReportResult{
			Source: kyvernoreports.SourceValidatingPolicy,
			Policy: "require-labels",
			Rule:   "autogen-check-labels",
			Result: "fail",
			Subjects: []corev1.ObjectReference{
				{Kind: "CronJob", Name: "my-cronjob", Namespace: "default"},
			},
		}
		err := printException(&buf, result, 0, "yaml")
		require.NoError(t, err)

		var exception policiesv1beta1.PolicyException
		require.NoError(t, yaml.Unmarshal(bytes.TrimPrefix(buf.Bytes(), []byte("---\n")), &exception))

		require.Len(t, exception.Spec.PolicyRefs, 1)
		assert.Equal(t, "require-labels", exception.Spec.PolicyRefs[0].Name)
		assert.Equal(t, "ValidatingPolicy", exception.Spec.PolicyRefs[0].Kind)

		require.Len(t, exception.Spec.MatchConditions, 1)
		expression := exception.Spec.MatchConditions[0].Expression
		assert.Contains(t, expression, `object.kind in ["CronJob", "Job", "Pod"]`)
		assert.Contains(t, expression, `object.metadata.name == "my-cronjob"`)
		assert.Contains(t, expression, `object.metadata.name.startsWith("my-cronjob-")`)
		compileMatchExpression(t, expression)
	})
}
