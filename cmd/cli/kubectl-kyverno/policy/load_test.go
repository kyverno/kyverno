package policy

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/admissionregistration/v1alpha1"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		resourcePath string
		paths        []string
		wantErr      bool
	}{{
		name:         "cpol-limit-configmap-for-sa",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/cpol-limit-configmap-for-sa.yaml"},
		wantErr:      false,
	}, {
		name:         "invalid-schema",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/invalid-schema.yaml"},
		wantErr:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Load(tt.fs, tt.resourcePath, tt.paths...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestLoadWithKubectlValidate(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		resourcePath string
		paths        []string
		wantErr      bool
		checks       func(*testing.T, []kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy)
	}{{
		name:         "cpol-limit-configmap-for-sa",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/cpol-limit-configmap-for-sa.yaml"},
		wantErr:      false,
	}, {
		name:         "invalid-schema",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/invalid-schema.yaml"},
		wantErr:      true,
	}, {
		name:         "proper defaulting",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/check-image.yaml"},
		wantErr:      false,
		checks: func(t *testing.T, policies []kyvernov1.PolicyInterface, vaps []v1alpha1.ValidatingAdmissionPolicy) {
			assert.Len(t, policies, 1)
			policy := policies[0]
			assert.NotNil(t, policy)
			spec := policy.GetSpec()
			assert.NotNil(t, spec)
			assert.True(t, spec.ValidationFailureAction.Audit())
			assert.NotNil(t, spec.Background)
			assert.True(t, *spec.Background)
			assert.NotNil(t, spec.Admission)
			assert.True(t, *spec.Admission)
			rule := spec.Rules[0]
			assert.Len(t, rule.VerifyImages, 1)
			assert.True(t, rule.VerifyImages[0].Required)
			assert.True(t, rule.VerifyImages[0].MutateDigest)
			assert.True(t, rule.VerifyImages[0].VerifyDigest)
			assert.True(t, rule.VerifyImages[0].UseCache)
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies, vaps, err := LoadWithLoader(KubectlValidateLoader, tt.fs, tt.resourcePath, tt.paths...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checks != nil {
				tt.checks(t, policies, vaps)
			}
		})
	}
}
