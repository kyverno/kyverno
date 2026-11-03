package egress

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// chartValues is the slice of charts/kyverno/values.yaml this package cares about. The lists moved
// out of features.registryClient when the allowlist model was replaced: they are ConfigMap
// content now, not flags, so they live beside the other config keys.
type chartValues struct {
	Config struct {
		RegistryEgressDeniedHosts []string `json:"registryEgressDeniedHosts"`
		RegistryEgressDeniedIPs   []string `json:"registryEgressDeniedIPs"`
	} `json:"config"`
}

func readChartValues(t *testing.T) chartValues {
	t.Helper()
	raw, err := os.ReadFile("../../../charts/kyverno/values.yaml")
	require.NoError(t, err)
	var values chartValues
	require.NoError(t, yaml.Unmarshal(raw, &values))
	return values
}

// TestDefaultDenylistsMatchTheChart pins the two copies of each list to each other. The chart's
// copies are what an operator reads and edits; the Go slices are the fallback for an install that
// has no ConfigMap entry at all -- the CLI, a hand-rolled manifest, a unit test -- so an entry
// added to only one of them silently protects half the installed base. Order is compared too: the
// chart is the documented reading order, and keeping it identical makes a drift failure point at
// the entry that moved instead of reporting two unordered sets.
func TestDefaultDenylistsMatchTheChart(t *testing.T) {
	t.Parallel()
	values := readChartValues(t)
	require.NotEmpty(t, values.Config.RegistryEgressDeniedHosts,
		"config.registryEgressDeniedHosts is missing from the chart")
	require.NotEmpty(t, values.Config.RegistryEgressDeniedIPs,
		"config.registryEgressDeniedIPs is missing from the chart")
	require.Equal(t, DefaultDeniedHosts, values.Config.RegistryEgressDeniedHosts,
		"DefaultDeniedHosts and config.registryEgressDeniedHosts have drifted")
	require.Equal(t, DefaultDeniedIPs, values.Config.RegistryEgressDeniedIPs,
		"DefaultDeniedIPs and config.registryEgressDeniedIPs have drifted")
	// The chart's copies go through the same parser, so a malformed entry there fails here
	// rather than quietly denying nothing in a user's cluster.
	chart, err := New(Config{
		DeniedHosts: values.Config.RegistryEgressDeniedHosts,
		DeniedIPs:   values.Config.RegistryEgressDeniedIPs,
	})
	require.NoError(t, err)
	require.Len(t, chart.prefixes, len(values.Config.RegistryEgressDeniedIPs))
	require.Len(t, chart.hosts, len(values.Config.RegistryEgressDeniedHosts))
}

// TestShippedDefaultsParse proves every shipped default survives New. The lists are applied
// best-effort now -- a malformed entry is logged and skipped rather than failing construction --
// so a typo in a default would stop denying its range with nothing failing anywhere. Counting
// what actually landed is the only way that shows up.
func TestShippedDefaultsParse(t *testing.T) {
	t.Parallel()
	for _, entry := range DefaultDeniedIPs {
		_, err := parsePrefix(entry)
		require.NoError(t, err, "shipped denied IP %q does not parse, so it denies nothing", entry)
	}
	for _, entry := range DefaultDeniedHosts {
		_, err := normalizeHost(entry)
		require.NoError(t, err, "shipped denied host %q does not parse, so it denies nothing", entry)
	}
	policy, err := New(Config{})
	require.NoError(t, err)
	require.Len(t, policy.prefixes, len(DefaultDeniedIPs), "a shipped denied IP was skipped")
	require.Len(t, policy.hosts, len(DefaultDeniedHosts), "a shipped denied host was skipped or duplicated")
}
