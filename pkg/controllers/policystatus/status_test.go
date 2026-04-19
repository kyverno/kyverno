package policystatus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeAuthorizingPolicy(t *testing.T) {
	obj := map[string]interface{}{
		"apiVersion": "policies.kyverno.io/v1alpha1",
		"kind":       "AuthorizingPolicy",
		"metadata": map[string]interface{}{
			"name": "test-apol",
		},
	}

	decoded, err := decodeAuthorizingPolicy(obj)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	require.Equal(t, "AuthorizingPolicy", decoded.Kind)
	require.Equal(t, "test-apol", decoded.Name)
}
