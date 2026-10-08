package manifest

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	"github.com/sigstore/rekor/pkg/generated/client/pubkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func signedResource(t *testing.T) (unstructured.Unstructured, *k8smanifest.VerifyResourceOption) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "public.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0600))
	obj := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "signed-resource"},
		"data": map[string]interface{}{"value": "signed"},
	}}
	data, err := yaml.Marshal(obj.Object)
	require.NoError(t, err)
	digest := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	require.NoError(t, err)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err = writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	obj.SetAnnotations(map[string]string{
		"cosign.sigstore.dev/message":   base64.StdEncoding.EncodeToString(compressed.Bytes()),
		"cosign.sigstore.dev/signature": base64.StdEncoding.EncodeToString(sig),
	})
	vo := k8smanifest.AddDefaultConfig(&k8smanifest.VerifyResourceOption{})
	vo.KeyPath = path
	vo.DisableDryRun = true
	return obj, vo
}

func TestSignedManifestVerification(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		change   func(*unstructured.Unstructured)
		verified bool
		fails    bool
	}{
		{name: "valid", verified: true},
		{name: "tampered resource", change: func(obj *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(obj.Object, "changed", "data", "value")
		}},
		{name: "invalid signature", change: func(obj *unstructured.Unstructured) {
			annotations := obj.GetAnnotations()
			annotations["cosign.sigstore.dev/signature"] = "aW52YWxpZA=="
			obj.SetAnnotations(annotations)
		}, fails: true},
		{name: "valid second signature", change: func(obj *unstructured.Unstructured) {
			annotations := obj.GetAnnotations()
			annotations["cosign.sigstore.dev/signature_1"] = annotations["cosign.sigstore.dev/signature"]
			annotations["cosign.sigstore.dev/signature"] = "aW52YWxpZA=="
			obj.SetAnnotations(annotations)
		}, verified: true},
		{name: "missing signature", change: func(obj *unstructured.Unstructured) {
			annotations := obj.GetAnnotations()
			delete(annotations, "cosign.sigstore.dev/signature")
			obj.SetAnnotations(annotations)
		}, fails: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			obj, vo := signedResource(t)
			if test.change != nil {
				test.change(&obj)
			}
			result, err := VerifyResource(context.Background(), obj, vo)
			if test.fails {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.verified, result.Verified)
		})
	}
}

func TestKeyedManifestPreservesSignerPatterns(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		signer   string
		verified bool
	}{
		{name: "wildcard matches empty keyed signer", signer: "*", verified: true},
		{name: "final signer check still rejects mismatch", signer: "other@example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			obj, vo := signedResource(t)
			vo.Signers = k8smanifest.SignerList{test.signer}
			// Legacy keyed identities skip the inner exact identity check and
			// apply SignerList.Match to the empty signer after verification.
			require.Equal(t, test.verified, vo.Signers.Match(""))
			result, err := VerifyResource(context.Background(), obj, vo)
			require.NoError(t, err)
			assert.Empty(t, result.Signer)
			assert.Equal(t, test.verified, result.Verified)
		})
	}
}

func TestManifestNetworkEndpointsAreGuarded(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	for _, field := range []string{"key", "image"} {
		t.Run(field, func(t *testing.T) {
			obj, vo := signedResource(t)
			if field == "key" {
				vo.KeyPath = server.URL + "/key.pub"
			} else {
				annotations := obj.GetAnnotations()
				annotations[vo.ResourceBundleRefAnnotationKey()] = server.Listener.Addr().String() + "/manifest:latest"
				obj.SetAnnotations(annotations)
			}
			_, err := VerifyResource(context.Background(), obj, vo)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			require.Zero(t, hits.Load())
		})
	}
}

func TestManifestRekorClientIsGuarded(t *testing.T) {
	_, vo := signedResource(t)
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", vo.KeyPath)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	vo.RekorURL = server.URL
	verifier, err := loadVerifier(context.Background(), vo.KeyPath)
	require.NoError(t, err)
	co, err := checkOptions(context.Background(), verifier, vo, false, true)
	require.NoError(t, err)
	_, err = co.RekorClient.Pubkey.GetPublicKey(pubkey.NewGetPublicKeyParamsWithContext(context.Background()))
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
	assert.Zero(t, hits.Load())
}

func TestManifestVerificationHonorsCancellation(t *testing.T) {
	t.Parallel()
	obj, vo := signedResource(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := VerifyResource(ctx, obj, vo)
	require.ErrorIs(t, err, context.Canceled)
}
