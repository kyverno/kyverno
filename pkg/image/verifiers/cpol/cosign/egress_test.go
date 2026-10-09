package cosign

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
)

func TestPolicySigstoreEndpointsUseEgressGuard(t *testing.T) {
	t.Run("Rekor URL", func(t *testing.T) {
		opts, err := buildCosignOptions(context.Background(), verifiers.Options{
			Client: registryclient.New(), Key: globalRekorPubKey,
			RekorURL: "http://169.254.169.254", RekorPubKey: globalRekorPubKey, IgnoreSCT: true,
		})
		require.NoError(t, err)
		_, err = opts.RekorClient.Tlog.GetLogInfo(nil)
		require.ErrorIs(t, err, egress.ErrAddressBlocked)
	})
	t.Run("key URL", func(t *testing.T) {
		_, err := buildCosignOptions(context.Background(), verifiers.Options{
			Client: registryclient.New(), Key: "http://169.254.169.254/key.pub", IgnoreTlog: true, IgnoreSCT: true,
		})
		require.ErrorIs(t, err, egress.ErrAddressBlocked)
	})
}
