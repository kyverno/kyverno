package egress

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// chartValues is the slice of charts/kyverno/values.yaml this package cares about.
type chartValues struct {
	Features struct {
		RegistryClient struct {
			PrivateRegistryRanges []string `json:"privateRegistryRanges"`
		} `json:"registryClient"`
	} `json:"features"`
}

// TestDefaultPrivateRangesMatchTheChart pins the two copies of the private-class list to each
// other. The chart's copy is what an operator reads and edits; DefaultPrivateRanges is what a
// Kyverno installed without the chart falls back to, and it is also the default of
// --privateRegistryRanges, so a range added to only one of them silently protects half the
// installed base. Order is compared too: the chart is the documented reading order, and keeping
// it identical makes a drift failure point at the entry that moved.
func TestDefaultPrivateRangesMatchTheChart(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../charts/kyverno/values.yaml")
	require.NoError(t, err)
	var values chartValues
	require.NoError(t, yaml.Unmarshal(raw, &values))
	require.NotEmpty(t, values.Features.RegistryClient.PrivateRegistryRanges,
		"features.registryClient.privateRegistryRanges is missing from the chart")
	require.Equal(t, DefaultPrivateRanges, values.Features.RegistryClient.PrivateRegistryRanges,
		"DefaultPrivateRanges and features.registryClient.privateRegistryRanges have drifted")
}

// TestChartPrivateRangesAreValid proves the shipped chart values parse, so a malformed entry
// fails here rather than at controller startup in a user's cluster.
func TestChartPrivateRangesAreValid(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../charts/kyverno/values.yaml")
	require.NoError(t, err)
	var values chartValues
	require.NoError(t, yaml.Unmarshal(raw, &values))
	_, err = New(Config{PrivateRanges: values.Features.RegistryClient.PrivateRegistryRanges})
	require.NoError(t, err)
}
