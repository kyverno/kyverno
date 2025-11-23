package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_configuration_Load_DefaultAllowExistingViolations(t *testing.T) {
	tests := []struct {
		name                           string
		data                           map[string]string
		expectedAllowExistingViolations bool
	}{
		{
			name:                           "not set",
			data:                           map[string]string{},
			expectedAllowExistingViolations: false,
		},
		{
			name: "set to true",
			data: map[string]string{
				"defaultAllowExistingViolations": "true",
			},
			expectedAllowExistingViolations: true,
		},
		{
			name: "set to false",
			data: map[string]string{
				"defaultAllowExistingViolations": "false",
			},
			expectedAllowExistingViolations: false,
		},
		{
			name: "set to invalid value",
			data: map[string]string{
				"defaultAllowExistingViolations": "invalid",
			},
			expectedAllowExistingViolations: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kyverno",
					Namespace: "kyverno",
				},
				Data: tt.data,
			}
			c := NewDefaultConfiguration(false)
			c.Load(cm)
			assert.Equal(t, tt.expectedAllowExistingViolations, c.GetDefaultAllowExistingViolations())
		})
	}
}
