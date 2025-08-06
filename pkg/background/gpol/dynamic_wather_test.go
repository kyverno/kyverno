package gpol

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSyncWatchers(t *testing.T) {
	log := logr.Discard()
	client := dclient.NewEmptyFakeClient()

	wm := NewWatchManager(log, client)

	grs := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{},
		},
	}

	err := wm.SyncWatchers("valid-policy", grs)
	assert.Error(t, err)
}
