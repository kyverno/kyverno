package common

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

func Test_LoadCrdsFromPaths_KeepsEveryPath(t *testing.T) {
	data.InjectProcessor(data.NewCRDProcessor(nil))
	require.NoError(t, LoadCrdsFromPaths([]string{
		"../../_testdata/apply/test-3/crd/crd.yml",
		"../../../../../test/cli/test-mutating-policy/mutate-custom-crd/crds/widget-crd.yaml",
	}))
	mapper := restmapper.NewDiscoveryRESTMapper(data.GetProcessor().GetResourceGroups())
	for kind, plural := range map[string]string{"Foo": "foos", "Widget": "widgets"} {
		mapping, err := mapper.RESTMapping(schema.GroupKind{Group: "example.com", Kind: kind}, "v1")
		require.NoError(t, err, kind)
		assert.Equal(t, plural, mapping.Resource.Resource)
	}
}
