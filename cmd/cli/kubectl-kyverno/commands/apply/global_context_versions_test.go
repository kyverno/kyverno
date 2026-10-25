package apply

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractGlobalContextEntryVersions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		apiVersion string
		kind       string
		extracted  bool
	}{
		{name: "stable", apiVersion: "kyverno.io/v2", kind: "GlobalContextEntry", extracted: true},
		{name: "beta", apiVersion: "kyverno.io/v2beta1", kind: "GlobalContextEntry", extracted: true},
		{name: "alpha", apiVersion: "kyverno.io/v2alpha1", kind: "GlobalContextEntry", extracted: true},
		{name: "unsupported version", apiVersion: "kyverno.io/v99", kind: "GlobalContextEntry"},
		{name: "different group", apiVersion: "example.com/v2", kind: "GlobalContextEntry"},
		{name: "different kind", apiVersion: "kyverno.io/v2", kind: "PolicyException"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": tc.apiVersion,
				"kind":       tc.kind,
				"metadata": map[string]interface{}{
					"name":   "cache",
					"labels": map[string]interface{}{"example.com/keep": "value"},
				},
				"spec": map[string]interface{}{
					"kubernetesResource": map[string]interface{}{"version": "v1", "resource": "configmaps"},
					"projections":        []interface{}{map[string]interface{}{"name": "names", "jmesPath": "[].metadata.name"}},
				},
				"status": map[string]interface{}{"ready": false},
			}}
			original := input.DeepCopy()
			regular, entries, err := extractGlobalContextEntries([]*unstructured.Unstructured{input})
			require.NoError(t, err)
			require.Equal(t, original, input, "extraction must not change the caller's object")
			if !tc.extracted {
				require.Empty(t, entries)
				require.Equal(t, []*unstructured.Unstructured{input}, regular)
				return
			}
			require.Empty(t, regular)
			require.Len(t, entries, 1)
			entry := entries[0]
			require.Equal(t, "kyverno.io/v2", entry.APIVersion)
			require.Equal(t, "GlobalContextEntry", entry.Kind)
			require.Equal(t, "value", entry.Labels["example.com/keep"])
			require.Equal(t, "configmaps", entry.Spec.KubernetesResource.Resource)
			require.Equal(t, "[].metadata.name", entry.Spec.Projections[0].JMESPath)
			require.NotNil(t, entry.Status.Ready)
			require.False(t, *entry.Status.Ready)
		})
	}
}
func TestApplyGlobalContextServedVersionFixtures(t *testing.T) {
	t.Parallel()
	root := "../../../../../test/cli/apply/globalcontext-served-versions"
	for _, version := range []string{"v2", "v2beta1", "v2alpha1"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			config := ApplyCommandConfig{
				PolicyPaths:   []string{filepath.Join(root, "policy.yaml")},
				ResourcePaths: []string{filepath.Join(root, "resources-"+version+".yaml")},
			}
			counts, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
			require.NoError(t, err)
			require.Equal(t, &processor.ResultCounts{Pass: 2}, counts)
			require.Len(t, responses, 2)
		})
	}
}
