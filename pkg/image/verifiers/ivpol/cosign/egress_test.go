package cosign

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestPolicySigstoreEndpointsUseEgressGuard(t *testing.T) {
	t.Run("ctlog URL", func(t *testing.T) {
		rekor, _, _, err := getRekor(context.Background(), &v1beta1.CTLog{URL: "http://169.254.169.254"}, nil, nil)
		require.NoError(t, err)
		_, err = rekor.Tlog.GetLogInfo(nil)
		require.ErrorIs(t, err, egress.ErrAddressBlocked)
	})
	t.Run("TUF mirror", func(t *testing.T) {
		_, err := getTrustedRootFromTUF(context.Background(), &v1beta1.TUF{Mirror: "http://169.254.169.254"})
		require.ErrorIs(t, err, egress.ErrAddressBlocked)
	})
	t.Run("TUF root URL", func(t *testing.T) {
		cfg := &v1beta1.TUF{}
		cfg.Root.Path = "http://169.254.169.254/root.json"
		_, err := getTrustedRootFromTUF(context.Background(), cfg)
		require.ErrorIs(t, err, egress.ErrAddressBlocked)
	})
	t.Run("key URL", func(t *testing.T) {
		remoteOpts, nameOpts := baseOpts()
		_, err := checkOptions(context.Background(), &v1beta1.Cosign{
			Key:   &v1beta1.Key{KMS: "http://169.254.169.254/key.pub"},
			CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true, InsecureIgnoreSCT: true},
		}, remoteOpts, nameOpts, nil)
		require.ErrorIs(t, err, egress.ErrAddressBlocked)
	})
}

type countingSecretLister struct {
	corev1listers.SecretLister
	calls int
}

func (l *countingSecretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	l.calls++
	return l.SecretLister.Secrets(namespace)
}

func TestSignatureSourceSecretsGuardedBeforeLookup(t *testing.T) {
	lister := &countingSecretLister{
		SecretLister: corev1listers.NewSecretLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}
	opts, err := sourceRemoteOpts(lister, &v1beta1.Source{
		SignaturePullSecrets: []corev1.LocalObjectReference{{Name: "signature-secret"}},
	})
	require.NoError(t, err)
	ref, err := name.ParseReference("169.254.169.254/signatures:latest")
	require.NoError(t, err)
	opts = append(opts, remote.WithContext(context.Background()), remote.WithTransport(registryclient.EgressTransport()))
	_, err = remote.Head(ref, opts...)
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
	require.Zero(t, lister.calls, "blocked signature repositories must not invoke the source credential lookup")
}
