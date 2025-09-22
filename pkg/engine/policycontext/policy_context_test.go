package policycontext

import (
	"os"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	cfg = config.NewDefaultConfiguration(false)
	jp  = jmespath.New(cfg)
)

func Test_DefaultAllowExistingViolations(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		configMapData map[string]string
		expected      bool
	}{
		{
			name:     "default value without env or configmap",
			envValue: "",
			expected: true,
		},
		{
			name:     "env var set to false",
			envValue: "false",
			expected: false,
		},
		{
			name:     "env var set to true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "env var invalid value",
			envValue: "invalid",
			expected: true,
		},
		{
			name:     "configmap overrides env var",
			envValue: "false",
			configMapData: map[string]string{
				"defaultAllowExistingViolations": "true",
			},
			expected: true,
		},
		{
			name:     "configmap invalid value falls back to default true",
			envValue: "false",
			configMapData: map[string]string{
				"defaultAllowExistingViolations": "invalid",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				t.Logf("Setting env var KYVERNO_DEFAULT_ALLOW_EXISTING_VIOLATIONS to %s", tt.envValue)
				os.Setenv("KYVERNO_DEFAULT_ALLOW_EXISTING_VIOLATIONS", tt.envValue)
				defer os.Unsetenv("KYVERNO_DEFAULT_ALLOW_EXISTING_VIOLATIONS")
			}
			// Verify env var is set
			if val := os.Getenv("KYVERNO_DEFAULT_ALLOW_EXISTING_VIOLATIONS"); val != tt.envValue {
				t.Errorf("Environment variable not set correctly. Got %s, want %s", val, tt.envValue)
			}

			// Create configuration
			cfg := config.NewDefaultConfiguration(false)

			// Apply configmap if provided
			if tt.configMapData != nil {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kyverno",
						Namespace: "kyverno",
					},
					Data: tt.configMapData,
				}
				cfg.Load(cm)
			}

			// Create policy context
			resource, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"}}`))
			assert.NoError(t, err)

			pc, err := NewPolicyContext(jp, *resource, kyvernov1.Create, nil, cfg)
			assert.NoError(t, err)

			// Verify configuration
			assert.Equal(t, tt.expected, pc.Config().GetDefaultAllowExistingViolations())
		})
	}
}

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
