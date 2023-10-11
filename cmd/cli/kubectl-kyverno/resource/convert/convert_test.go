package convert

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestTo(t *testing.T) {
	{
		data, err := os.ReadFile("../../_testdata/policies/check-image.yaml")
		require.NoError(t, err)

		json, err := yaml.YAMLToJSON(data)
		require.NoError(t, err)

		var untyped unstructured.Unstructured
		require.NoError(t, untyped.UnmarshalJSON(json))

		typed, err := To[corev1.ConfigMap](untyped)
		require.Nil(t, typed)
		require.Error(t, err)
	}
	{
		data, err := os.ReadFile("../../_testdata/resources/namespace.yaml")
		require.NoError(t, err)

		json, err := yaml.YAMLToJSON(data)
		require.NoError(t, err)

		var untyped unstructured.Unstructured
		require.NoError(t, untyped.UnmarshalJSON(json))

		typed, err := To[corev1.Namespace](untyped)
		require.NotNil(t, typed)
		require.NoError(t, err)
	}
}
