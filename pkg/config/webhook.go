package config

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebhookConfig struct {
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty" protobuf:"bytes,5,opt,name=namespaceSelector"`
	ObjectSelector    *metav1.LabelSelector `json:"objectSelector,omitempty" protobuf:"bytes,11,opt,name=objectSelector"`
}

func parseWebhooks(webhooks string) ([]WebhookConfig, error) {
	webhookCfgs := make([]WebhookConfig, 0, 10)
	if err := json.Unmarshal([]byte(webhooks), &webhookCfgs); err != nil {
		return nil, err
	}

	return webhookCfgs, nil
}
