package evaluator

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func Test_Compile_VerifyDigest_DefaultsToTrue(t *testing.T) {
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
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := ivpol.DeepCopy()
			p.Spec.ValidationConfigurations = tt.cfg

			compiled, errs := NewCompiler(
				nil,
				nil,
				nil,
				imageverifycache.DisabledImageVerifyCache(),
			).Compile(p, nil)

			require.Empty(t, errs)

			cp, ok := compiled.(*compiledPolicy)
			require.True(t, ok)

			assert.Equal(t, tt.want, cp.verifyDigest)
		})
	}
}
