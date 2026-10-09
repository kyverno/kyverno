package credentials

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type contextualSecretLister struct {
	corev1listers.SecretLister
	namespace *contextualSecretNamespaceLister
}

func (l contextualSecretLister) Secrets(string) corev1listers.SecretNamespaceLister {
	return l.namespace
}

type contextualSecretNamespaceLister struct {
	corev1listers.SecretNamespaceLister
	get func(context.Context, string) (*corev1.Secret, error)
}

func (l *contextualSecretNamespaceLister) GetWithContext(ctx context.Context, name string) (*corev1.Secret, error) {
	return l.get(ctx, name)
}

type credentialCalls struct{ count int }

func (k *credentialCalls) Resolve(authn.Resource) (authn.Authenticator, error) {
	k.count++
	return authn.Anonymous, nil
}

func TestSecretKeychainPreservesCancellationAndForbidden(t *testing.T) {
	ref, err := name.ParseReference("registry.example/image:tag")
	require.NoError(t, err)
	t.Run("caller cancellation reaches live GET", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan struct{})
		lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(ctx context.Context, _ string) (*corev1.Secret, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}}}
		result := make(chan error, 1)
		go func() {
			_, err := authn.Resolve(ctx, NewSecretsKeychain(lister, "tenant", logr.Discard(), "creds"), ref.Context())
			result <- err
		}()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("secret lookup did not start")
		}
		cancel()
		select {
		case err := <-result:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("secret lookup ignored cancellation")
		}
	})
	for _, test := range []struct {
		name     string
		err      error
		fallback bool
	}{
		{"forbidden stops fallback", apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "creds", errors.New("denied")), false},
		{"missing secret preserves fallback", apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "creds"), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(context.Context, string) (*corev1.Secret, error) { return nil, test.err }}}
			fallback := &credentialCalls{}
			_, err := authn.Resolve(context.Background(), authn.NewMultiKeychain(NewSecretsKeychain(lister, "tenant", logr.Discard(), "creds"), fallback), ref.Context())
			if test.fallback {
				require.NoError(t, err)
				require.Equal(t, 1, fallback.count)
			} else {
				require.True(t, apierrors.IsForbidden(err))
				require.Zero(t, fallback.count)
			}
		})
	}
}
