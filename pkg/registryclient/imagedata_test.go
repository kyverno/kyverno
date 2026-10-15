package registryclient

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestImageDataOptionsPreserveRuntimeCancellation(t *testing.T) {
	calls := 0
	lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(context.Context, string) (*corev1.Secret, error) {
		calls++
		return nil, errors.New("cancelled request must not read credentials")
	}}}
	opts, names := ImageDataOptsFromImageVerificationCredentials(lister, policiesv1alpha1.Credentials{Secrets: []string{"registry"}}, "tenant")
	images, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = images.Get(ctx, "192.0.2.1/image:tag", opts, names)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls)
}

func TestImageDataOptionsRetainConfiguredSecretReferences(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("stop after credential lookup")
	var requested string
	lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(_ context.Context, name string) (*corev1.Secret, error) {
		requested = name
		return nil, lookupErr
	}}}
	references := []string{"original"}
	configured := New(WithSecretLister(lister, "tenant"), WithImagePullSecrets(references...)).(*client)
	references[0] = "changed-after-construction"

	// Reusing image options does not rerun constructor options or allow a caller
	// to alter another caller's option slice. Secret contents still refresh in SDK.
	first, _ := configured.imageDataOptions()
	first[0] = remote.WithAuth(nil)
	opts, names := configured.imageDataOptions()
	ref, err := name.ParseReference("192.0.2.1/image:tag", names...)
	require.NoError(t, err)
	transport := &offlineCredentialTransport{}
	_, err = remote.Get(ref, append(opts, remote.WithTransport(transport), remote.WithContext(context.Background()))...)
	require.ErrorIs(t, err, lookupErr)
	require.Equal(t, "original", requested)
	require.Zero(t, transport.calls)
}
