package registryclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

func TestCredentialLookupPreservesCancellation(test *testing.T) {
	for _, source := range []string{"configured Secret", "nested keychains", "supplied keychain"} {
		test.Run(source, func(test *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			entered := make(chan context.Context, 1)
			release := make(chan struct{})
			defer close(release)
			lookup := func(lookupContext context.Context) error {
				entered <- lookupContext
				select {
				case <-lookupContext.Done():
					return lookupContext.Err()
				case <-release:
					return errors.New("credential lookup outlived the request")
				}
			}
			reference := name.MustParseReference("192.0.2.1/image:tag")
			transport := &offlineCredentialTransport{}
			var resolve func() error
			if source == "supplied keychain" {
				configured := New(WithEgressMode("enforce")).(*client)
				configured.transport = transport
				useResourceImageClient(test, configured)
				keychain := &contextKeychain{resolve: func(lookupContext context.Context, _ authn.Resource) (authn.Authenticator, error) {
					return nil, lookup(lookupContext)
				}}
				options, err := GuardResourceImageOptions(ctx, reference.Name(), []remote.Option{remote.WithAuthFromKeychain(keychain), remote.WithContext(context.Background())})
				require.NoError(test, err)
				resolve = func() error {
					_, err := remote.Get(reference, options...)
					return err
				}
			} else {
				lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(lookupContext context.Context, _ string) (*corev1.Secret, error) {
					return nil, lookup(lookupContext)
				}}}
				options := []Option{WithSecretLister(lister, "tenant"), WithImagePullSecrets("creds")}
				if source == "nested keychains" {
					options = append(options, WithKeychain(&contextKeychain{}), WithKeychain(&contextKeychain{}))
				}
				configured := New(options...)
				resolve = func() error {
					_, err := authn.Resolve(ctx, configured.Keychain(), reference.Context())
					return err
				}
			}
			result := make(chan error, 1)
			go func() { result <- resolve() }()
			select {
			case lookupContext := <-entered:
				require.Same(test, ctx, lookupContext)
				require.NoError(test, lookupContext.Err())
			case <-time.After(5 * time.Second):
				test.Fatal("credential lookup did not start")
			}
			cancel()
			select {
			case err := <-result:
				require.ErrorIs(test, err, context.Canceled)
			case <-time.After(5 * time.Second):
				test.Fatal("credential lookup ignored request cancellation")
			}
			require.Empty(test, transport.requests)
		})
	}
}

func TestImageDataOptionsPreserveRuntimeCancellation(test *testing.T) {
	calls := 0
	lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(context.Context, string) (*corev1.Secret, error) {
		calls++
		return nil, errors.New("cancelled request must not read credentials")
	}}}
	options, names := ImageDataOptsFromImageVerificationCredentials(lister, policiesv1alpha1.Credentials{Secrets: []string{"registry"}}, "tenant")
	images, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(test, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = images.Get(ctx, "192.0.2.1/image:tag", options, names)
	require.ErrorIs(test, err, context.Canceled)
	require.Zero(test, calls)
}

func TestImageDataOptionsRetainConfiguredSecretReferences(test *testing.T) {
	test.Parallel()
	lookupErr := errors.New("stop after credential lookup")
	var requested string
	lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(_ context.Context, secretName string) (*corev1.Secret, error) {
		requested = secretName
		return nil, lookupErr
	}}}
	references := []string{"original"}
	configured := New(WithSecretLister(lister, "tenant"), WithImagePullSecrets(references...)).(*client)
	references[0] = "changed-after-construction"
	first, _ := configured.imageDataOptions()
	first[0] = remote.WithAuth(nil)
	options, names := configured.imageDataOptions()
	reference, err := name.ParseReference("192.0.2.1/image:tag", names...)
	require.NoError(test, err)
	transport := &offlineCredentialTransport{}
	_, err = remote.Get(reference, append(options, remote.WithTransport(transport), remote.WithContext(context.Background()))...)
	require.ErrorIs(test, err, lookupErr)
	require.Equal(test, "original", requested)
	require.Empty(test, transport.requests)
}

func TestOfflinePolicySecretCredentialsRequireBackend(test *testing.T) {
	test.Parallel()
	credentials := policiesv1alpha1.Credentials{Secrets: []string{"tenant/registry"}}
	options, _ := OptsFromImageVerificationCredentials(context.Background(), nil, credentials, "kyverno")
	imageOptions, _ := ImageDataOptsFromImageVerificationCredentials(nil, credentials, "kyverno")
	for label, options := range map[string][]remote.Option{"registry": options, "image data": imageOptions} {
		test.Run(label, func(test *testing.T) {
			test.Parallel()
			transport := &offlineCredentialTransport{}
			options = append(options, remote.WithTransport(transport), remote.WithContext(context.Background()))
			_, err := remote.Get(name.MustParseReference("192.0.2.1/image:latest"), options...)
			require.ErrorContains(test, err, "secret lister is not configured")
			require.Empty(test, transport.requests)
		})
	}
}
