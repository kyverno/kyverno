package internal

import (
	"flag"
	"os"
	"os/exec"
	"testing"

	guardedregistry "github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/registryclient"
	"github.com/stretchr/testify/require"
)

func TestValidateRegistryClientConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		allowlist string
		mode      string
		wantError bool
	}{
		{name: "defaults"},
		{name: "audit", mode: "audit"},
		{name: "enforce", mode: "enforce", allowlist: "registry.internal,10.0.0.0/8,fd00::/8"},
		{name: "invalid mode", mode: "disabled", wantError: true},
		{name: "wildcard", allowlist: "*.internal", wantError: true},
		{name: "URL", allowlist: "https://registry.internal", wantError: true},
		{name: "invalid CIDR", allowlist: "10.0.0.0/33", wantError: true},
		{name: "empty list element", allowlist: "registry.internal,,10.0.0.1", wantError: true},
		{name: "trailing comma", allowlist: "registry.internal,", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateRegistryClientConfig(tc.allowlist, tc.mode)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateRegistryClientConfig() error = %v, want error = %v", err, tc.wantError)
			}
		})
	}
}

func TestRegistryEgressFlags(t *testing.T) {
	originalFlags := flag.CommandLine
	originalAllowlist, originalMode := privateRegistryAllowlist, privateRegistryEgressMode
	originalSecrets, originalHelpers := imagePullSecrets, registryCredentialHelpers
	originalInsecure, originalPayloadLimit := allowInsecureRegistry, imageVerifyPayloadLimit
	t.Cleanup(func() {
		flag.CommandLine = originalFlags
		privateRegistryAllowlist, privateRegistryEgressMode = originalAllowlist, originalMode
		imagePullSecrets, registryCredentialHelpers = originalSecrets, originalHelpers
		allowInsecureRegistry, imageVerifyPayloadLimit = originalInsecure, originalPayloadLimit
	})
	for _, test := range []struct {
		name       string
		options    []ConfigurationOption
		wantEgress bool
		wantClient bool
	}{
		{name: "disabled"},
		{name: "egress only", options: []ConfigurationOption{WithRegistryEgress()}, wantEgress: true},
		{name: "registry client", options: []ConfigurationOption{WithRegistryClient()}, wantEgress: true, wantClient: true},
		{name: "both options", options: []ConfigurationOption{WithRegistryEgress(), WithRegistryClient()}, wantEgress: true, wantClient: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(test.name, flag.ContinueOnError)
			configuration := NewConfiguration(test.options...)
			require.Equal(t, test.wantEgress, configuration.UsesRegistryEgress())
			require.Equal(t, test.wantClient, configuration.UsesRegistryClient())
			initRegistryFlags(configuration)
			for _, flagName := range []string{"privateRegistryAllowlist", "privateRegistryEgressMode"} {
				require.Equal(t, test.wantEgress, flag.Lookup(flagName) != nil, flagName)
			}
			for _, flagName := range []string{"imagePullSecrets", "registryCredentialHelpers", "allowInsecureRegistry", "imageVerifyPayloadLimit"} {
				require.Equal(t, test.wantClient, flag.Lookup(flagName) != nil, flagName)
			}
			if test.wantEgress {
				require.Equal(t, "audit", privateRegistryEgressMode)
				require.Empty(t, privateRegistryAllowlist)
				require.NoError(t, flag.CommandLine.Parse([]string{"--privateRegistryEgressMode=enforce", "--privateRegistryAllowlist=10.20.30.0/24"}))
				require.Equal(t, "enforce", privateRegistryEgressMode)
				require.Equal(t, "10.20.30.0/24", privateRegistryAllowlist)
			}
		})
	}
}

func TestSetupRegistryEgress(t *testing.T) {
	if os.Getenv("KYVERNO_TEST_REGISTRY_EGRESS") != "1" {
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSetupRegistryEgress$", "-test.v")
		command.Env = append(os.Environ(), "KYVERNO_TEST_REGISTRY_EGRESS=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}

	privateRegistryEgressMode = "enforce"
	privateRegistryAllowlist = "10.20.30.0/24,169.254.169.254"
	setupRegistryEgress()
	_, err := registryclient.GetRegistryClient()
	require.Error(t, err)
	_, err = guardedregistry.GetRegistryClient()
	require.NoError(t, err)
	require.NoError(t, guardedregistry.ValidateResourceImage(t.Context(), "10.20.30.1/image:tag"))
	require.ErrorIs(t, guardedregistry.ValidateResourceImage(t.Context(), "10.20.31.1/image:tag"), guardedregistry.ErrRegistryAddressBlocked)
	require.ErrorIs(t, guardedregistry.ValidateResourceImage(t.Context(), "169.254.169.254/image:tag"), guardedregistry.ErrRegistryAddressBlocked)
}
