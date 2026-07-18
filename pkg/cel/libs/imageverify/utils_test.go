package imageverify

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestArrToMap(t *testing.T) {
	tests := []struct {
		name string
		arr  []v1beta1.Attestation
		want map[string]v1beta1.Attestation
	}{{
		name: "nil slice",
		arr:  nil,
		want: map[string]v1beta1.Attestation{},
	}, {
		name: "empty slice",
		arr:  []v1beta1.Attestation{},
		want: map[string]v1beta1.Attestation{},
	}, {
		name: "single attestation",
		arr: []v1beta1.Attestation{{
			Name: "sbom",
		}},
		want: map[string]v1beta1.Attestation{
			"sbom": {Name: "sbom"},
		},
	}, {
		name: "multiple attestations",
		arr: []v1beta1.Attestation{{
			Name: "sbom",
		}, {
			Name: "provenance",
		}},
		want: map[string]v1beta1.Attestation{
			"sbom":       {Name: "sbom"},
			"provenance": {Name: "provenance"},
		},
	}, {
		name: "duplicate key last wins",
		arr: []v1beta1.Attestation{{
			Name: "sbom",
		}, {
			Name: "sbom",
		}},
		want: map[string]v1beta1.Attestation{
			"sbom": {Name: "sbom"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := arrToMap(tt.arr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func makeImagePolicy(attestations []v1beta1.Attestation) *v1beta1.ImageValidatingPolicy {
	return &v1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1beta1.ImageValidatingPolicySpec{
			Attestations: attestations,
		},
	}
}

func TestAttestationMap(t *testing.T) {
	tests := []struct {
		name   string
		policy v1beta1.ImageValidatingPolicyLike
		want   map[string]v1beta1.Attestation
	}{{
		name:   "nil policy",
		policy: nil,
		want:   nil,
	}, {
		name:   "policy with no attestations",
		policy: makeImagePolicy(nil),
		want:   map[string]v1beta1.Attestation{},
	}, {
		name: "policy with attestations",
		policy: makeImagePolicy([]v1beta1.Attestation{{
			Name: "sbom",
		}, {
			Name: "provenance",
		}}),
		want: map[string]v1beta1.Attestation{
			"sbom":       {Name: "sbom"},
			"provenance": {Name: "provenance"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attestationMap(tt.policy)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRemoteOptsFromPolicy(t *testing.T) {
	tests := []struct {
		name          string
		creds         *v1beta1.Credentials
		wantNonEmpty  bool
	}{{
		name:         "nil credentials",
		creds:        nil,
		wantNonEmpty: false,
	}, {
		name:         "empty credentials",
		creds:        &v1beta1.Credentials{},
		wantNonEmpty: false,
	}, {
		name: "with secrets",
		creds: &v1beta1.Credentials{
			Secrets: []string{"my-secret"},
		},
		wantNonEmpty: true,
	}, {
		name: "with providers",
		creds: &v1beta1.Credentials{
			Providers: []v1beta1.CredentialsProvidersType{"google"},
		},
		wantNonEmpty: true,
	}, {
		name: "with insecure registry",
		creds: &v1beta1.Credentials{
			AllowInsecureRegistry: true,
		},
		wantNonEmpty: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRemoteOptsFromPolicy(tt.creds)
			if tt.wantNonEmpty {
				assert.NotEmpty(t, got)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}
