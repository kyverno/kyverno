package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttestor_GetKey(t *testing.T) {
	tests := []struct {
		name     string
		attestor Attestor
		want     string
	}{{
		name: "foo",
		attestor: Attestor{
			Name: "foo",
		},
		want: "foo",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestor.GetKey()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestor_IsCosign(t *testing.T) {
	tests := []struct {
		name     string
		attestor Attestor
		want     bool
	}{{
		name:     "no",
		attestor: Attestor{},
		want:     false,
	}, {
		name: "yes",
		attestor: Attestor{
			Cosign: &Cosign{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestor.IsCosign()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestor_IsNotary(t *testing.T) {
	tests := []struct {
		name     string
		attestor Attestor
		want     bool
	}{{
		name:     "no",
		attestor: Attestor{},
		want:     false,
	}, {
		name: "yes",
		attestor: Attestor{
			Notary: &Notary{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestor.IsNotary()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestation_GetKey(t *testing.T) {
	tests := []struct {
		name        string
		attestation Attestation
		want        string
	}{{
		name: "foo",
		attestation: Attestation{
			Name: "foo",
		},
		want: "foo",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestation.GetKey()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestation_IsInToto(t *testing.T) {
	tests := []struct {
		name        string
		attestation Attestation
		want        bool
	}{{
		name:        "no",
		attestation: Attestation{},
		want:        false,
	}, {
		name: "yes",
		attestation: Attestation{
			InToto: &InToto{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestation.IsInToto()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestation_IsReferrer(t *testing.T) {
	tests := []struct {
		name        string
		attestation Attestation
		want        bool
	}{{
		name:        "no",
		attestation: Attestation{},
		want:        false,
	}, {
		name: "yes",
		attestation: Attestation{
			Referrer: &Referrer{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestation.IsReferrer()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageVerificationPolicySpec_EvaluationMode(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicySpec
		want   EvaluationMode
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicySpec{},
		want:   EvaluationModeKubernetes,
	}, {
		name: "json",
		policy: &ImageVerificationPolicySpec{
			EvaluationConfiguration: &EvaluationConfiguration{
				Mode: EvaluationModeJSON,
			},
		},
		want: EvaluationModeJSON,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.EvaluationMode()
			assert.Equal(t, tt.want, got)
		})
	}
}
