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

func TestValidateContainersFail(t *testing.T) {

	resource := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"image": "nginx",
					"name":  "front-end",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
				},
				map[string]interface{}{
					"image": "nickchase/rss-php-nginx:v1",
					"name":  "rss-reader",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
				},
			},
		},
	}

	pattern := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"image": "nginx",
					"name":  "front-end",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "50m",
							"memory": "120Mi",
						},
					},
				},
				map[string]interface{}{
					"image": "nickchase/rss-php-nginx:v1",
					"name":  "rss-reader",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "50m",
							"memory": "120Mi",
						},
					},
				},
			},
		},
	}

	var log logr.Logger
	_, err := ValidateResourceWithPattern(log, resource, pattern)
	assert.Assert(t, err != nil)
}

func TestValidateContainersPass(t *testing.T) {

	resource := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"image": "nginx",
					"name":  "front-end",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
				},
				map[string]interface{}{
					"image": "nickchase/rss-php-nginx:v1",
					"name":  "rss-reader",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
				},
			},
		},
	}

	pattern := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"image": "nginx",
					"name":  "front-end",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "50m",
							"memory": "120Mi",
						},
					},
				},
				map[string]interface{}{
					"image": "nickchase/rss-php-nginx:v1",
					"name":  "rss-reader",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 3000,
						},
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "50m",
							"memory": "120Mi",
						},
					},
				},
			},
		},
	}

	var log logr.Logger
	_, err := ValidateResourceWithPattern(log, resource, pattern)
	assert.Assert(t, err != nil)
}