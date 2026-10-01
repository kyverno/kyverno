package imageverify

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestFactoryCredentialsObserveSecretRotation(t *testing.T) {
	var expected atomic.Value
	expected.Store("first")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "user" || password != expected.Load().(string) {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("a", 64))
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: config.KyvernoNamespace()}, Type: corev1.SecretTypeDockerConfigJson}
	update := func(password string) {
		updated := secret.DeepCopy()
		updated.Data = map[string][]byte{corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, host, base64.StdEncoding.EncodeToString([]byte("user:"+password))))}
		require.NoError(t, indexer.Update(updated))
	}
	update("first")
	policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{Credentials: &policiesv1beta1.Credentials{Secrets: []string{"registry"}, AllowInsecureRegistry: true}}}
	factory := NewFactory(logr.Discard(), policy, corev1listers.NewSecretLister(indexer), types.DefaultTypeAdapter, nil)
	ref, err := name.ParseReference(host+"/image:tag", factory.functions.nameOpts...)
	require.NoError(t, err)
	check := func() error {
		opts := append([]remote.Option{}, factory.functions.authOpts...)
		opts = append(opts, remote.WithContext(context.Background()))
		_, err := remote.Head(ref, opts...)
		return err
	}
	require.NoError(t, check())
	expected.Store("second")
	require.Error(t, check(), "the registry must reject the old credentials")
	update("second")
	require.NoError(t, check(), "the existing compiled factory must use the updated Secret")
}
