package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfiguration_GetWebhookValidating(t *testing.T) {
	tests := []struct {
		name                string
		webhook             WebhookConfig
		webhookValidating   WebhookConfig
		expectedNSSelector  *metav1.LabelSelector
		expectedObjSelector *metav1.LabelSelector
	}{
		{
			name:                "both configs empty",
			webhook:             WebhookConfig{},
			webhookValidating:   WebhookConfig{},
			expectedNSSelector:  nil,
			expectedObjSelector: nil,
		},
		{
			name: "only general webhook has values",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			},
			webhookValidating:   WebhookConfig{},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		},
		{
			name:    "only webhookValidating has values",
			webhook: WebhookConfig{},
			webhookValidating: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"version": "v1"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"version": "v1"}},
		},
		{
			name: "webhookValidating takes precedence",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			},
			webhookValidating: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"version": "v1"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"version": "v1"}},
		},
		{
			name: "partial override - only namespace selector in webhookValidating",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			},
			webhookValidating: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		},
		{
			name: "partial override - only object selector in webhookValidating",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			},
			webhookValidating: WebhookConfig{
				ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"version": "v1"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"version": "v1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration{
				webhook:           tt.webhook,
				webhookValidating: tt.webhookValidating,
			}

			originalWebhook := config.webhook
			originalWebhookValidating := config.webhookValidating

			result := config.GetWebhookValidating()

			assert.Equal(t, tt.expectedNSSelector, result.NamespaceSelector)
			assert.Equal(t, tt.expectedObjSelector, result.ObjectSelector)

			assert.Equal(t, originalWebhook, config.webhook)
			assert.Equal(t, originalWebhookValidating, config.webhookValidating)
		})
	}
}

func TestConfiguration_GetWebhookMutating(t *testing.T) {
	tests := []struct {
		name                string
		webhook             WebhookConfig
		webhookMutating     WebhookConfig
		expectedNSSelector  *metav1.LabelSelector
		expectedObjSelector *metav1.LabelSelector
	}{
		{
			name:                "both configs empty",
			webhook:             WebhookConfig{},
			webhookMutating:     WebhookConfig{},
			expectedNSSelector:  nil,
			expectedObjSelector: nil,
		},
		{
			name: "only general webhook has values",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"component": "api"}},
			},
			webhookMutating:     WebhookConfig{},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"component": "api"}},
		},
		{
			name:    "only webhookMutating has values",
			webhook: WebhookConfig{},
			webhookMutating: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "us-west"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"type": "deployment"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "us-west"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"type": "deployment"}},
		},
		{
			name: "webhookMutating takes precedence",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"component": "api"}},
			},
			webhookMutating: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "us-west"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"type": "deployment"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "us-west"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"type": "deployment"}},
		},
		{
			name: "partial override - only namespace selector in webhookMutating",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"component": "api"}},
			},
			webhookMutating: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "us-west"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "us-west"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"component": "api"}},
		},
		{
			name: "partial override - only object selector in webhookMutating",
			webhook: WebhookConfig{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
				ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"component": "api"}},
			},
			webhookMutating: WebhookConfig{
				ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"type": "deployment"}},
			},
			expectedNSSelector:  &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
			expectedObjSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"type": "deployment"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration{
				webhook:         tt.webhook,
				webhookMutating: tt.webhookMutating,
			}

			originalWebhook := config.webhook
			originalWebhookMutating := config.webhookMutating

			result := config.GetWebhookMutating()

			assert.Equal(t, tt.expectedNSSelector, result.NamespaceSelector)
			assert.Equal(t, tt.expectedObjSelector, result.ObjectSelector)

			assert.Equal(t, originalWebhook, config.webhook)
			assert.Equal(t, originalWebhookMutating, config.webhookMutating)
		})
	}
}
