package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseResourceRequestOperation_create(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("CREATE")

	assert.NoError(t, err)
	assert.Equal(t, ResourceCreated, result)
}

func TestParseResourceRequestOperation_update(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("UPDATE")

	assert.NoError(t, err)
	assert.Equal(t, ResourceUpdated, result)
}

func TestParseResourceRequestOperation_delete(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("DELETE")

	assert.NoError(t, err)
	assert.Equal(t, ResourceDeleted, result)
}

func TestParseResourceRequestOperation_connect(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("CONNECT")

	assert.NoError(t, err)
	assert.Equal(t, ResourceConnected, result)
}

func TestParseResourceRequestOperation_invalid_operation(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("INVALID")

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "unknown request operation")
}

func TestParseResourceRequestOperation_lowercase_rejected(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("create")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestParseResourceRequestOperation_empty_string(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestGetResourceKindAndNamespace_NilResource(t *testing.T) {
	t.Parallel()

	kind, namespace := GetResourceKindAndNamespace(nil)
	assert.Equal(t, "", kind)
	assert.Equal(t, "", namespace)
}

func TestGetResourceKindAndNamespace_NonNilResource(t *testing.T) {
	t.Parallel()

	resource := &unstructured.Unstructured{}
	resource.SetKind("Pod")
	resource.SetNamespace("default")

	kind, namespace := GetResourceKindAndNamespace(resource)
	assert.Equal(t, "Pod", kind)
	assert.Equal(t, "default", namespace)
}
