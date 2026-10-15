package registryclient

import (
	"context"
	"errors"
	"testing"

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
