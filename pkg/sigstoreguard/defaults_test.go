package sigstoreguard

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
)

func TestTUFUsesOperatorDefault(t *testing.T) {
	mu.Lock()
	oldMirror, oldRoot := defaultMirror, defaultRoot
	mu.Unlock()
	t.Cleanup(func() { SetDefaultRepository(oldMirror, oldRoot) })
	t.Setenv("TUF_ROOT", t.TempDir())
	SetDefaultRepository("http://169.254.169.254", nil)
	_, err := TrustedRootFor(context.Background(), "", nil)
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
	_, err = VerificationMaterialFor(context.Background(), "", nil)
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
}
