package cosign

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// TestCheckOptions_SignaturePullSecretsApplyOverReusedPuller guards that a
// source's signaturePullSecrets still authenticate when the image's options
// carry a reused puller, as the ones imageverify binds per evaluation do. A
// puller authenticates with the options it was built from and ignores any
// keychain appended after it, so without clearing it the signature repository
// would be reached with the policy's own credentials instead.
func TestCheckOptions_SignaturePullSecretsApplyOverReusedPuller(t *testing.T) {
	var requireAuth atomic.Bool
	inner := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); requireAuth.Load() && (!ok || user != "sig" || password != "pw") {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")

	ref, err := name.ParseReference(host+"/signatures/image:tag", name.Insecure)
	require.NoError(t, err)
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, img))
	requireAuth.Store(true)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	require.NoError(t, indexer.Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "signatures", Namespace: config.KyvernoNamespace()},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`,
			host, base64.StdEncoding.EncodeToString([]byte("sig:pw"))))},
	}))

	// the image's options as imageverify binds them: a puller built from the
	// policy's own credentials, which have no access to the signature repository
	puller, err := remote.NewPuller()
	require.NoError(t, err)
	baseROpts := []remote.Option{remote.Reuse(puller)}

	cosignCfg := &v1beta1.Cosign{
		Key:    &v1beta1.Key{Data: testPublicKey},
		CTLog:  &v1beta1.CTLog{InsecureIgnoreTlog: true},
		Source: &v1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: "signatures"}}},
	}
	opts, err := checkOptions(context.Background(), cosignCfg, baseROpts, []name.Option{name.Insecure}, corev1listers.NewSecretLister(indexer))
	require.NoError(t, err)

	_, err = ociremote.SignedEntity(ref, opts.RegistryClientOpts...)
	require.NoError(t, err, "the signature pull secret must authenticate despite the reused puller")
}
