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
	// Verify default value is initialized in constructor
	assert.Equal(t, DefaultMaxContextSize, cfg.GetMaxContextSize())

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

func TestConfiguration_GetUpdateRequestCleanupTTL_InvalidValue(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno",
			Namespace: "kyverno",
		},
		Data: map[string]string{
			"updateRequestCleanupTTL": "invalid-ttl",
		},
	}

	cfg.Load(cm)

	// Should fall back to empty string on parse error
	assert.Equal(t, "", cfg.GetUpdateRequestCleanupTTL())
}

// TestConfiguration_GetUpdateRequestCleanupTTL_LabelValue pins that a TTL which
// parses as a duration but is not a valid label value is rejected at load. The
// value is applied as the cleanup.kyverno.io/ttl label on every UpdateRequest,
// so accepting one would make the API server reject every UpdateRequest create
// and silently stop generate and mutate-existing rules.
func TestConfiguration_GetUpdateRequestCleanupTTL_LabelValue(t *testing.T) {
	tests := []struct {
		name     string
		ttl      string
		expected string
	}{
		// strfmt.ParseDuration accepts these spelled-out forms, but a space is
		// not allowed in a label value.
		{name: "spelled-out days", ttl: "7 days", expected: ""},
		{name: "spelled-out hour", ttl: "1 hour", expected: ""},
		{name: "spelled-out weeks", ttl: "2 weeks", expected: ""},
		// Valid durations and the supported date formats are label-safe and
		// must keep working.
		{name: "hours", ttl: "24h", expected: "24h"},
		{name: "minutes", ttl: "30m", expected: "30m"},
		{name: "date", ttl: "2026-10-26", expected: "2026-10-26"},
		{name: "datetime", ttl: "2026-10-26T120000Z", expected: "2026-10-26T120000Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefaultConfiguration(false)
			cfg.Load(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
				Data:       map[string]string{"updateRequestCleanupTTL": tt.ttl},
			})
			assert.Equal(t, tt.expected, cfg.GetUpdateRequestCleanupTTL())
		})
	}
}

func TestConfiguration_GetUpdateRequestCleanupTTL_Unload(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{"updateRequestCleanupTTL": "1h"},
	})
	assert.Equal(t, "1h", cfg.GetUpdateRequestCleanupTTL())

	// unload should reset to empty string
	cfg.Load(nil)
	assert.Equal(t, "", cfg.GetUpdateRequestCleanupTTL())
}

func TestConfiguration_GetUpdateRequestThreshold(t *testing.T) {
	cfg := NewDefaultConfiguration(false)
	assert.Equal(t, int64(UpdateRequestThreshold), cfg.GetUpdateRequestThreshold())

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data: map[string]string{
			"updateRequestThreshold": "500",
		},
	}
	cfg.Load(cm)
	assert.Equal(t, int64(500), cfg.GetUpdateRequestThreshold())

	// Invalid value falls back to default
	cmInvalid := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data: map[string]string{
			"updateRequestThreshold": "invalid",
		},
	}
	cfg.Load(cmInvalid)
	assert.Equal(t, int64(UpdateRequestThreshold), cfg.GetUpdateRequestThreshold())
}

func TestConfiguration_Unload_ResetsUpdateRequestThresholdAndMatchConditions(t *testing.T) {
	cfg := NewDefaultConfiguration(false)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data: map[string]string{
			"updateRequestThreshold": "250",
			"matchConditions":        `[{"name":"test-condition","expression":"true"}]`,
		},
	}
	cfg.Load(cm)
	assert.Equal(t, int64(250), cfg.GetUpdateRequestThreshold())
	assert.Len(t, cfg.GetMatchConditions(), 1)

	// Unload by passing nil
	cfg.Load(nil)
	assert.Equal(t, int64(UpdateRequestThreshold), cfg.GetUpdateRequestThreshold())
	assert.Nil(t, cfg.GetMatchConditions())
}
