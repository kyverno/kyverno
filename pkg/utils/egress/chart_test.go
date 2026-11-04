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

// shippedConfig is the denylist an install gets from the chart. There is no copy in Go, so tests
// that need the recommended defaults read them from here.
func shippedConfig(t *testing.T) Config {
	t.Helper()
	values := readChartValues(t)
	return Config{DeniedHosts: values.Config.RegistryEgressDeniedHosts, DeniedIPs: values.Config.RegistryEgressDeniedIPs}
}

// A malformed entry is logged and skipped rather than failing New, so a typo in the chart would
// stop denying its range with nothing failing anywhere. Counting what landed catches it.
func TestShippedDenylistsParse(t *testing.T) {
	t.Parallel()
	shipped := shippedConfig(t)
	require.NotEmpty(t, shipped.DeniedHosts, "config.registryEgressDeniedHosts is missing from the chart")
	require.NotEmpty(t, shipped.DeniedIPs, "config.registryEgressDeniedIPs is missing from the chart")
	for _, entry := range shipped.DeniedIPs {
		_, err := parsePrefix(entry)
		require.NoError(t, err, "shipped denied IP %q does not parse, so it denies nothing", entry)
	}
	for _, entry := range shipped.DeniedHosts {
		_, err := normalizeHost(entry)
		require.NoError(t, err, "shipped denied host %q does not parse, so it denies nothing", entry)
	}
	policy, err := New(shipped)
	require.NoError(t, err)
	require.Len(t, policy.prefixes, len(shipped.DeniedIPs), "a shipped denied IP was skipped")
	require.Len(t, policy.hosts, len(shipped.DeniedHosts), "a shipped denied host was skipped or duplicated")
}
