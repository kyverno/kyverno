package autogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestControllerNames(t *testing.T) {
	// ControllerNames reports exactly the built-in controllers ResolveConfig
	// accepts without a cluster, sorted so callers see a stable list.
	assert.Equal(t, []string{
		"cronjobs",
		"daemonsets",
		"deployments",
		"jobs",
		"replicasets",
		"statefulsets",
	}, ControllerNames())
	for _, name := range ControllerNames() {
		assert.NotNil(t, ConfigsMap[name])
	}
}
