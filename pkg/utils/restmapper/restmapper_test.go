package restmapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

type mockDiscoveryWithMapper struct {
	dclient.IDiscovery
	mapper meta.RESTMapper
}

func (m *mockDiscoveryWithMapper) RESTMapper() meta.RESTMapper {
	return m.mapper
}

type mockClientWithDiscovery struct {
	dclient.Interface
	disco dclient.IDiscovery
}

func (m *mockClientWithDiscovery) Discovery() dclient.IDiscovery {
	return m.disco
}

func TestGetRESTMapper_WithClientMapper(t *testing.T) {
	expectedMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	client := &mockClientWithDiscovery{
		disco: &mockDiscoveryWithMapper{
			mapper: expectedMapper,
		},
	}

	mapper, err := GetRESTMapper(client)
	assert.NoError(t, err)
	assert.Same(t, expectedMapper, mapper)
}

func TestGetRESTMapper_FallbackForFakeClient(t *testing.T) {
	client := dclient.NewEmptyFakeClient()
	mapper, err := GetRESTMapper(client)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}
