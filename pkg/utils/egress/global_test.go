package egress

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetGlobal restores whatever Guard was installed, so these tests can run alongside others
// in the package without leaking process-wide state.
func resetGlobal(t *testing.T) {
	t.Helper()
	previous := global.Load()
	t.Cleanup(func() { global.Store(previous) })
}

func TestGlobalNilGuardLeavesSDKDefaults(t *testing.T) {
	resetGlobal(t)
	global.Store(nil)

	guard := Global()
	require.Nil(t, guard)
	// A nil Guard must stay usable: registryclient.WithTransport(nil) falls back to the SDK
	// default, which is what the CLI and unit tests want.
	require.Nil(t, guard.Transport())
	require.Nil(t, guard.Policy())
}

func TestSetGlobalRejectsOnlyAnInvalidMode(t *testing.T) {
	resetGlobal(t)
	global.Store(nil)

	require.Error(t, SetGlobal(Config{Mode: "disabled"}))
	require.Nil(t, Global(), "a rejected mode must leave the previous Guard in place")
	// A malformed denylist entry is ConfigMap content, not a deploy-time flag, so it is logged
	// and skipped. Failing here would crashloop the admission controller over a typo and take
	// the cluster's admission path down with it.
	require.NoError(t, SetGlobal(Config{Mode: ModeEnforce, DeniedIPs: []string{"10.0.0.0/99", "10.0.0.0/8"}}))
	require.NotNil(t, Global().Transport())
	require.NotNil(t, Global().Policy())
	// The transport must be built from the Guard's own policy. Two policies here would mean the
	// destinations the transport enforces differ from the ones Policy() reports.
	require.Same(t, Global().Policy(), Global().Transport().(*Transport).policy)
}

func TestRestoreGlobalReinstallsNoGuard(t *testing.T) {
	resetGlobal(t)
	global.Store(nil)

	require.NoError(t, SetGlobal(Config{}))
	installed := Global()
	require.NotNil(t, installed)
	// SetGlobal builds a Guard from configuration and so cannot express "no Guard at all". A
	// caller that installs one temporarily needs this to put the process back exactly as it
	// found it, including when it found nothing.
	RestoreGlobal(nil)
	require.Nil(t, Global())
	RestoreGlobal(installed)
	require.Same(t, installed, Global())
}

func TestValidateMode(t *testing.T) {
	t.Parallel()
	// The mode is the one piece of this configuration fixed at deploy time, so a controller can
	// fail startup on a typo instead of discovering it on the first registry request. Everything
	// else arrives later, from a ConfigMap, and is deliberately not checked here.
	for _, mode := range []string{"", ModeAudit, ModeEnforce} {
		require.NoError(t, ValidateMode(mode), "mode %q", mode)
	}
	for _, mode := range []string{"off", "disabled", "warn", "Audit", "ENFORCE", " audit"} {
		require.Error(t, ValidateMode(mode), "mode %q", mode)
	}
}
