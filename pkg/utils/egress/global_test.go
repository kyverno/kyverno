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
	// default.
	require.Nil(t, guard.Transport())
	require.Nil(t, guard.Policy())
}

func TestSetGlobalRejectsInvalidConfig(t *testing.T) {
	resetGlobal(t)
	require.Error(t, SetGlobal(Config{Mode: "disabled"}))
	require.Error(t, SetGlobal(Config{Allowlist: []string{"*.internal"}}))
	require.NoError(t, SetGlobal(Config{Mode: ModeEnforce, Allowlist: []string{"10.0.0.0/8"}}))
	require.NotNil(t, Global().Transport())
	require.NotNil(t, Global().Policy())
}

func TestValidateConfigMatchesNew(t *testing.T) {
	t.Parallel()
	for _, config := range []Config{
		{},
		{Mode: ModeAudit},
		{Mode: ModeEnforce, Allowlist: []string{"registry.internal", "10.0.0.0/8"}},
		{Mode: "nonsense"},
		{Allowlist: []string{""}},
	} {
		_, newErr := New(config)
		require.Equal(t, newErr != nil, ValidateConfig(config) != nil, "config %+v", config)
	}
}

func TestParseAllowlist(t *testing.T) {
	t.Parallel()
	require.Nil(t, ParseAllowlist(""))
	require.Nil(t, ParseAllowlist("   "))
	require.Equal(t, []string{"a", "b"}, ParseAllowlist("a,b"))
	// A single empty element is preserved so New rejects it rather than silently ignoring it.
	require.Equal(t, []string{"a", "", "b"}, ParseAllowlist("a,,b"))
}
