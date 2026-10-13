package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeVerifier struct {
	token, audience, group string
	err                    error
}

func (f *fakeVerifier) Verify(_ context.Context, token, audience, group string) error {
	f.token, f.audience, f.group = token, audience, group
	return f.err
}

func TestReceiverAudience(t *testing.T) {
	r := &Receiver{name: "kyverno-svc", namespace: "kyverno", port: 8443}
	require.Equal(t, "https://kyverno-svc.kyverno.svc:8443/validate/fail", r.Audience("/validate/fail"))
	require.Equal(t, "https://kyverno-svc.kyverno.svc:8443/validate/fail/", r.Audience("/validate/fail/"))
	r.port = 0
	require.Equal(t, "https://kyverno-svc.kyverno.svc:443/", r.Audience(""))
	r.server = "webhook.example:9443"
	require.Equal(t, "https://webhook.example:9443/vpol/a/b", r.Audience("/vpol/a/b"))
}

func TestReceiverHeaders(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers []string
		valid   bool
	}{
		{"missing", nil, false},
		{"empty", []string{"Bearer "}, false},
		{"basic", []string{"Basic secret"}, false},
		{"duplicate", []string{"Bearer one", "Bearer two"}, false},
		{"comma", []string{"Bearer one,two"}, false},
		{"extra", []string{"Bearer one two"}, false},
		{"valid", []string{"Bearer signed.token"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeVerifier{}
			r := &Receiver{Verifier: fake, name: "svc", namespace: "ns", port: 443}
			request := httptest.NewRequest("POST", "https://untrusted.example/vpol/policy", nil)
			request.Host = "attacker.example"
			request.Header.Set("X-Forwarded-Host", "attacker.example")
			for _, h := range tc.headers {
				request.Header.Add("Authorization", h)
			}
			err := r.VerifyRequest(request, "policies.kyverno.io")
			if !tc.valid {
				require.Error(t, err)
				require.Empty(t, fake.token)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "signed.token", fake.token)
			require.Equal(t, "https://svc.ns.svc:443/vpol/policy", fake.audience)
			require.Equal(t, "policies.kyverno.io", fake.group)
			fake.err = errors.New("signature invalid")
			require.Error(t, r.VerifyRequest(request, "policies.kyverno.io"))
		})
	}
}

func TestReceiverRejectsEncodedOrQueriedPaths(t *testing.T) {
	r := &Receiver{Verifier: &fakeVerifier{}, name: "svc", namespace: "ns", port: 443}
	for _, target := range []string{"/vpol/%70olicy", "/vpol/policy?mode=test"} {
		request := httptest.NewRequest("POST", target, nil)
		request.Header.Set("Authorization", "Bearer token")
		require.Error(t, r.VerifyRequest(request, "apps"))
	}
}
