package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestKubernetesSignedTokens(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "key-1", Algorithm: "RS256", Use: "sig"}}}
	var keyCalls atomic.Int32
	var keysUnavailable atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": "https://issuer.example", "jwks_uri": "https://untrusted.example/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/openid/v1/jwks":
			keyCalls.Add(1)
			if keysUnavailable.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(keys)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config := &rest.Config{Host: server.URL, TLSClientConfig: rest.TLSClientConfig{CAData: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})}}
	v, err := NewVerifier(context.Background(), config)
	require.NoError(t, err)
	const audience = "https://kyverno-svc.kyverno.svc:443/validate/fail"
	claims := func() map[string]any {
		return map[string]any{
			"iss": "https://issuer.example", "sub": "system:serviceaccount:kyverno:caller",
			"aud": []string{audience}, "exp": time.Now().Add(time.Minute).Unix(),
			"kubernetes.io": map[string]any{"attestations": map[string]any{"admissionReviewAPIGroups": []string{"apps"}}},
		}
	}
	signWithKid := func(k *rsa.PrivateKey, kid string, body map[string]any) string {
		t.Helper()
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: k}, (&jose.SignerOptions{}).WithHeader("kid", kid))
		require.NoError(t, err)
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		jws, err := signer.Sign(payload)
		require.NoError(t, err)
		token, err := jws.CompactSerialize()
		require.NoError(t, err)
		return token
	}
	sign := func(k *rsa.PrivateKey, body map[string]any) string { return signWithKid(k, "key-1", body) }
	valid := sign(key, claims())
	require.NoError(t, v.Verify(context.Background(), valid, audience, "apps"))
	require.EqualValues(t, 1, keyCalls.Load())
	require.NoError(t, v.Verify(context.Background(), valid, audience, "apps"))
	require.EqualValues(t, 1, keyCalls.Load(), "known keys should use the shared cache")
	for _, tc := range []struct {
		name     string
		change   func(map[string]any)
		group    string
		audience string
		key      *rsa.PrivateKey
		ok       bool
	}{
		{"wrong signature", nil, "apps", audience, other, false},
		{"wrong issuer", func(c map[string]any) { c["iss"] = "https://elsewhere.example" }, "apps", audience, key, false},
		{"wrong service same path", nil, "apps", "https://other.kyverno.svc:443/validate/fail", key, false},
		{"expired", func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() }, "apps", audience, key, false},
		{"missing claim", func(c map[string]any) { delete(c, "kubernetes.io") }, "apps", audience, key, false},
		{"missing attestations", func(c map[string]any) { c["kubernetes.io"] = map[string]any{} }, "apps", audience, key, false},
		{"missing groups", func(c map[string]any) { c["kubernetes.io"] = map[string]any{"attestations": map[string]any{}} }, "apps", audience, key, false},
		{"multiple groups", func(c map[string]any) {
			c["kubernetes.io"] = map[string]any{"attestations": map[string]any{"admissionReviewAPIGroups": []string{"apps", "batch"}}}
		}, "apps", audience, key, false},
		{"wrong group", nil, "batch", audience, key, false},
		{"empty core group", func(c map[string]any) {
			c["kubernetes.io"] = map[string]any{"attestations": map[string]any{"admissionReviewAPIGroups": []string{""}}}
		}, "", audience, key, false},
		{"wildcard core", func(c map[string]any) {
			c["kubernetes.io"] = map[string]any{"attestations": map[string]any{"admissionReviewAPIGroups": []string{"*"}}}
		}, "", audience, key, true},
		{"wildcard wrong audience", func(c map[string]any) {
			c["kubernetes.io"] = map[string]any{"attestations": map[string]any{"admissionReviewAPIGroups": []string{"*"}}}
		}, "apps", "https://other.kyverno.svc:443/validate/fail", key, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := claims()
			if tc.change != nil {
				tc.change(body)
			}
			err := v.Verify(context.Background(), sign(tc.key, body), tc.audience, tc.group)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	require.Error(t, v.Verify(context.Background(), "malformed", audience, "apps"))
	keys.Keys = append(keys.Keys, jose.JSONWebKey{Key: &other.PublicKey, KeyID: "key-2", Algorithm: "RS256", Use: "sig"})
	require.NoError(t, v.Verify(context.Background(), signWithKid(other, "key-2", claims()), audience, "apps"))
	require.Greater(t, keyCalls.Load(), int32(1), "unknown key should refresh the shared JWKS cache")
	keysUnavailable.Store(true)
	fresh, err := NewVerifier(context.Background(), config)
	require.NoError(t, err)
	require.Error(t, fresh.Verify(context.Background(), valid, audience, "apps"), "unavailable keys must fail closed")
}

func TestDiscoveryFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://elsewhere.example/", http.StatusFound)
		}},
		{"invalid json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{")) }},
		{"missing issuer", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"jwks_uri":"https://elsewhere.example/keys"}`))
		}},
		{"unavailable", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(tc.handler)
			defer server.Close()
			config := &rest.Config{Host: server.URL, TLSClientConfig: rest.TLSClientConfig{CAData: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})}}
			_, err := NewVerifier(context.Background(), config)
			require.Error(t, err)
		})
	}
}

func TestDiscoveryTimeoutIsBounded(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	config := &rest.Config{Host: server.URL, TLSClientConfig: rest.TLSClientConfig{CAData: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})}}
	start := time.Now()
	_, err := NewVerifier(context.Background(), config)
	require.Error(t, err)
	require.Less(t, time.Since(start), 7*time.Second)
}
