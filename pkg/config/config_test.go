package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfiguration_GetMaxContextSize_Default(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	// Load with nil configmap to trigger unload which sets defaults
	cfg.Load(nil)

	assert.Equal(t, DefaultMaxContextSize, cfg.GetMaxContextSize())
}

func TestConfiguration_GetMaxContextSize_FromConfigMap(t *testing.T) {
	cfg := NewDefaultConfiguration(false)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno",
			Namespace: "kyverno",
		},
		Data: map[string]string{
			"maxContextSize": "4194304", // 4MB
		},
	}

	cfg.Load(cm)

	assert.Equal(t, int64(4194304), cfg.GetMaxContextSize())
}

func TestConfiguration_GetMaxContextSize_InvalidValue(t *testing.T) {
	cfg := NewDefaultConfiguration(false)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno",
			Namespace: "kyverno",
		},
		Data: map[string]string{
			"maxContextSize": "invalid",
		},
	}

	cfg.Load(cm)

	// Should fall back to default on parse error
	assert.Equal(t, DefaultMaxContextSize, cfg.GetMaxContextSize())
}

func TestConfiguration_GetMaxContextSize_NotSet(t *testing.T) {
	cfg := NewDefaultConfiguration(false)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno",
			Namespace: "kyverno",
		},
		Data: map[string]string{},
	}

	cfg.Load(cm)

	// Should use default when not set
	assert.Equal(t, DefaultMaxContextSize, cfg.GetMaxContextSize())
}

func TestConfiguration_GetMaxContextSize_ZeroDisablesLimit(t *testing.T) {
	cfg := NewDefaultConfiguration(false)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno",
			Namespace: "kyverno",
		},
		Data: map[string]string{
			"maxContextSize": "0",
		},
	}

	cfg.Load(cm)

	// Zero should be valid and disable the limit
	assert.Equal(t, int64(0), cfg.GetMaxContextSize())
}

func TestConfiguration_GetGenerateMutationEvents_Default(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	assert.False(t, cfg.GetGenerateMutationEvents())
}

func TestConfiguration_GetGenerateMutationEvents_Unload(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{"generateMutationEvents": "true"},
	})
	assert.True(t, cfg.GetGenerateMutationEvents())

	// unload should reset to false
	cfg.Load(nil)
	assert.False(t, cfg.GetGenerateMutationEvents())
}

func TestConfiguration_GetGenerateMutationEvents_FromConfigMap(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true", "true", true},
		{"false", "false", false},
		{"True", "True", true},
		{"invalid falls back to default", "notabool", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefaultConfiguration(false)
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
				Data:       map[string]string{"generateMutationEvents": tt.value},
			}
			cfg.Load(cm)
			assert.Equal(t, tt.expected, cfg.GetGenerateMutationEvents())
		})
	}
}

func TestConfiguration_GetGenerateMutationEvents_NotSet(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{},
	}
	cfg.Load(cm)
	assert.False(t, cfg.GetGenerateMutationEvents())
}

func TestConfiguration_GetMaxContextSize_KubernetesQuantityFormat(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int64
	}{
		{"100Mi", "100Mi", 100 * 1024 * 1024},
		{"4Mi", "4Mi", 4 * 1024 * 1024},
		{"1Gi", "1Gi", 1024 * 1024 * 1024},
		{"500Ki", "500Ki", 500 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefaultConfiguration(false)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kyverno",
					Namespace: "kyverno",
				},
				Data: map[string]string{
					"maxContextSize": tt.value,
				},
			}

			cfg.Load(cm)

			assert.Equal(t, tt.expected, cfg.GetMaxContextSize())
		})
	}
}
