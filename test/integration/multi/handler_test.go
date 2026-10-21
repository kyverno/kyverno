//go:build integration

// Package multi_test demonstrates the option-based TestEnv API by wiring
// multiple policy types into a single envtest binary. It is intentionally
// minimal: the goal is to prove that NewTestEnvWithOptions correctly composes
// per-type setups and that engines for different policy types coexist without
// cross-pollution.
package multi_test

import (
	"os"
	"testing"

	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/require"
)

var testEnv *framework.TestEnv

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnvWithOptions(
		framework.WithPolicyTypes(framework.Vpol, framework.Mpol, framework.Gpol),
		framework.WithExceptionsEnabled(),
	)
	if err != nil {
		panic(err)
	}

	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}

// TestMulti_OnlyRequestedTypesAreWired asserts that NewTestEnvWithOptions
// populates exactly the types requested via WithPolicyTypes. Types that were
// not requested must remain nil so callers can detect misconfigured tests
// up-front instead of nil-dereferencing later.
func TestMulti_OnlyRequestedTypesAreWired(t *testing.T) {
	require.NotNil(t, testEnv.Vpol, "Vpol was requested but not wired")
	require.NotNil(t, testEnv.Vpol.Engine)
	require.NotNil(t, testEnv.Vpol.Provider)

	require.NotNil(t, testEnv.Mpol, "Mpol was requested but not wired")
	require.NotNil(t, testEnv.Mpol.Engine)
	require.NotNil(t, testEnv.Mpol.Provider)

	require.NotNil(t, testEnv.Gpol, "Gpol was requested but not wired")
	require.NotNil(t, testEnv.Gpol.Engine)
	require.NotNil(t, testEnv.Gpol.Provider)
	require.NotNil(t, testEnv.Gpol.Lister)
	require.NotNil(t, testEnv.Gpol.NamespacedLister)

	require.Nil(t, testEnv.Dpol, "Dpol was not requested and must be nil")
}
