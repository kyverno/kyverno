package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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

func TestConfiguration_GetSuccessEventActions_Default(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	assert.Equal(t, 0, cfg.GetSuccessEventActions().Len())
}

func TestConfiguration_GetSuccessEventActions_Unload(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{"successEventActions": "Resource Mutated"},
	})
	assert.True(t, cfg.GetSuccessEventActions().Has("Resource Mutated"))

	// unload should reset to empty
	cfg.Load(nil)
	assert.Equal(t, 0, cfg.GetSuccessEventActions().Len())
}

func TestConfiguration_GetSuccessEventActions_FromConfigMap(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected sets.Set[string]
	}{
		{"single action", "Resource Mutated", sets.New("Resource Mutated")},
		{"multiple actions", "Resource Mutated,Resource Passed", sets.New("Resource Mutated", "Resource Passed")},
		{"whitespace trimmed", " Resource Mutated , Resource Passed ", sets.New("Resource Mutated", "Resource Passed")},
		{"empty string", "", sets.New[string]()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefaultConfiguration(false)
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
				Data:       map[string]string{"successEventActions": tt.value},
			}
			cfg.Load(cm)
			assert.Equal(t, tt.expected, cfg.GetSuccessEventActions())
		})
	}
}

func TestConfiguration_GetSuccessEventActions_NotSet(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{},
	}
	cfg.Load(cm)
	assert.Equal(t, 0, cfg.GetSuccessEventActions().Len())
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
