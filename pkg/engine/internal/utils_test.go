package internal

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

const singleKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAExample1==
-----END PUBLIC KEY-----`

const twoKeys = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAExample1==
-----END PUBLIC KEY-----
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAExample2==
-----END PUBLIC KEY-----`

func TestExpandStaticKeys(t *testing.T) {
	tests := []struct {
		name         string
		attestorSet  kyvernov1.AttestorSet
		wantEntries  int
	}{
		{
			name: "no entries",
			attestorSet: kyvernov1.AttestorSet{
				Entries: nil,
			},
			wantEntries: 0,
		},
		{
			name: "entry without keys",
			attestorSet: kyvernov1.AttestorSet{
				Entries: []kyvernov1.Attestor{
					{Certificates: &kyvernov1.CertificateAttestor{}},
				},
			},
			wantEntries: 1,
		},
		{
			name: "single PEM key — no split",
			attestorSet: kyvernov1.AttestorSet{
				Entries: []kyvernov1.Attestor{
					{Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: singleKey}},
				},
			},
			wantEntries: 1,
		},
		{
			name: "two PEM keys — splits into two entries",
			attestorSet: kyvernov1.AttestorSet{
				Entries: []kyvernov1.Attestor{
					{Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: twoKeys}},
				},
			},
			wantEntries: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandStaticKeys(tt.attestorSet)
			if len(got.Entries) != tt.wantEntries {
				t.Errorf("ExpandStaticKeys() entries = %d, want %d", len(got.Entries), tt.wantEntries)
			}
		})
	}
}
