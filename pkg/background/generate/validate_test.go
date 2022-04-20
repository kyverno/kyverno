package generate

import (
	"testing"

	"github.com/go-logr/logr"
	"gotest.tools/assert"
)

func TestValidatePass(t *testing.T) {
	resource := map[string]interface{}{
		"spec": map[string]interface{}{
			"egress": map[string]interface{}{
				"port": []interface{}{
					map[string]interface{}{
						"port":     5353,
						"protocol": "UDP",
					},
					map[string]interface{}{
						"port":     5353,
						"protocol": "TCP",
					},
				},
			},
		},
	}
	pattern := map[string]interface{}{
		"spec": map[string]interface{}{
			"egress": map[string]interface{}{
				"port": []interface{}{
					map[string]interface{}{
						"port":     5353,
						"protocol": "UDP",
					},
					map[string]interface{}{
						"port":     5353,
						"protocol": "TCP",
					},
				},
			},
		},
	}

	var log logr.Logger
	_, err := ValidateResourceWithPattern(log, resource, pattern)
	assert.NilError(t, err)
}

func TestValidateFail(t *testing.T) {
	resource := map[string]interface{}{
		"spec": map[string]interface{}{
			"egress": map[string]interface{}{
				"port": []interface{}{
					map[string]interface{}{
						"port":     5353,
						"protocol": "TCP",
					},
				},
			},
		},
	}
	pattern := map[string]interface{}{
		"spec": map[string]interface{}{
			"egress": map[string]interface{}{
				"port": []interface{}{
					map[string]interface{}{
						"port":     5353,
						"protocol": "UDP",
					},
					map[string]interface{}{
						"port":     5353,
						"protocol": "TCP",
					},
				},
			},
		},
	}

	var log logr.Logger
	_, err := ValidateResourceWithPattern(log, resource, pattern)
	assert.Assert(t, err != nil)
}
