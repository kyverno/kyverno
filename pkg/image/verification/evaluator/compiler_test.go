package evaluator

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func Test_Compile_VerifyDigest_Resolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  policiesv1alpha1.ValidationConfiguration
		want bool
	}{
		{
			name: "unset defaults to enabled",
			cfg:  policiesv1alpha1.ValidationConfiguration{},
			want: true,
		},
		{
			name: "explicit true",
			cfg: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: ptr.To(true),
			},
			want: true,
		},
		{
			name: "explicit false",
			cfg: policiesv1alpha1.ValidationConfiguration{
				VerifyDigest: ptr.To(false),
			},
			want: false,
		},
		{
			name: "mutateDigest false with unset verifyDigest defaults to false",
			cfg: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(false),
			},
			want: false,
		},
		{
			name: "mutateDigest false with explicit verifyDigest true",
			cfg: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(false),
				VerifyDigest: ptr.To(true),
			},
			want: true,
		},
		{
			name: "mutateDigest false with explicit verifyDigest false",
			cfg: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(false),
				VerifyDigest: ptr.To(false),
			},
			want: false,
		},
		{
			name: "mutateDigest true with unset verifyDigest defaults to true",
			cfg: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(true),
			},
			want: true,
		},
		{
			name: "mutateDigest true with explicit verifyDigest true",
			cfg: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(true),
				VerifyDigest: ptr.To(true),
			},
			want: true,
		},
		{
			name: "mutateDigest true with explicit verifyDigest false",
			cfg: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(true),
				VerifyDigest: ptr.To(false),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := ivpol.DeepCopy()
			p.Spec.ValidationConfigurations = tt.cfg

			compiled, errs := NewCompiler(nil).Compile(p, nil)

			require.Empty(t, errs)

			cp, ok := compiled.(*compiledPolicy)
			require.True(t, ok)

			assert.Equal(t, tt.want, cp.verifyDigest)
		})
	}
}

func Test_Compile_SerializedPolicy_VerifyDigest(t *testing.T) {
	t.Parallel()

	rawYAML := `
apiVersion: policies.kyverno.io/v1alpha1
kind: ImageValidatingPolicy
metadata:
  name: test-mutate-false-no-verify
spec:
  validationConfigurations:
    mutateDigest: false
`
	var policy policiesv1beta1.ImageValidatingPolicy
	err := yaml.Unmarshal([]byte(rawYAML), &policy)
	require.NoError(t, err)

	compiled, errList := NewCompiler(nil).Compile(&policy, nil)
	require.Empty(t, errList)

	cp, ok := compiled.(*compiledPolicy)
	require.True(t, ok)
	assert.False(t, cp.verifyDigest, "deserialized policy with mutateDigest: false and omitted verifyDigest must resolve verifyDigest to false")
}
