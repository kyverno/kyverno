package policycontext

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
)

var (
	cfg = config.NewDefaultConfiguration(false)
	jp  = jmespath.New(cfg)
)

func Test_setResources(t *testing.T) {
	newResource, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "small"
		  },
		  "name": "namespace1"
		},
		"spec": {}
	  }`))
	assert.Nil(t, err)

	oldResource, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "small"
		  },
		  "name": "namespace2"
		},
		"spec": {}
	  }`))
	assert.Nil(t, err)

	pc, err := NewPolicyContext(jp, *newResource, kyvernov1.Update, nil, cfg)
	assert.Nil(t, err)
	pc = pc.WithOldResource(*oldResource)

	n := pc.NewResource()
	assert.Equal(t, "namespace1", n.GetName())

	o := pc.OldResource()
	assert.Equal(t, "namespace2", o.GetName())

	// swap resources
	pc.SetResources(*newResource, *oldResource)

	n = pc.NewResource()
	assert.Equal(t, "namespace2", n.GetName())

	name, err := pc.JSONContext().Query("request.object.metadata.name")
	assert.Nil(t, err)
	assert.Equal(t, "namespace2", name)

	o = pc.OldResource()
	assert.Equal(t, "namespace1", o.GetName())
	name, err = pc.JSONContext().Query("request.oldObject.metadata.name")
	assert.Nil(t, err)
	assert.Equal(t, "namespace1", name)

	// swap back resources
	pc.SetResources(*oldResource, *newResource)

	n = pc.NewResource()
	assert.Equal(t, "namespace1", n.GetName())

	name, err = pc.JSONContext().Query("request.object.metadata.name")
	assert.Nil(t, err)
	assert.Equal(t, "namespace1", name)

	o = pc.OldResource()
	assert.Equal(t, "namespace2", o.GetName())
	name, err = pc.JSONContext().Query("request.oldObject.metadata.name")
	assert.Nil(t, err)
	assert.Equal(t, "namespace2", name)
}
