package registryclient

import (
	"context"
	"errors"
	"testing"
	"time"

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
			_, err := authn.Resolve(ctx, NewSecretsKeychain(lister, "tenant", "creds"), ref.Context())
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
			_, err := authn.Resolve(context.Background(), authn.NewMultiKeychain(NewSecretsKeychain(lister, "tenant", "creds"), fallback), ref.Context())
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

func TestRegistryClientSecretLookupPreservesContext(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		layered bool
	}{
		{name: "configured Secret"},
		{name: "nested keychains", layered: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			entered := make(chan context.Context, 1)
			release := make(chan struct{})
			defer close(release)
			lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(ctx context.Context, _ string) (*corev1.Secret, error) {
				entered <- ctx
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
					return nil, errors.New("Secret lookup outlived the request")
				}
			}}}
			opts := []Option{WithSecretLister(lister, "tenant"), WithImagePullSecrets("creds")}
			if test.layered {
				opts = append(opts, WithKeychain(&credentialCalls{}), WithKeychain(&credentialCalls{}))
			}
			configured := New(opts...)
			// A public IP literal passes destination validation without DNS or a connection.
			ref := name.MustParseReference("8.8.8.8/image:tag")
			result := make(chan error, 1)
			go func() {
				_, err := authn.Resolve(ctx, configured.Keychain(), ref.Context())
				result <- err
			}()
			select {
			case lookupContext := <-entered:
				deadline, ok := lookupContext.Deadline()
				require.True(t, ok)
				expected, _ := ctx.Deadline()
				require.Equal(t, expected, deadline)
			case <-time.After(5 * time.Second):
				t.Fatal("assembled keychain did not start the Secret lookup")
			}
			cancel()
			select {
			case err := <-result:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("assembled keychain ignored request cancellation")
			}
		})
	}
}
