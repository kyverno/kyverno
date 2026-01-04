package cluster

import (
	"context"
	"errors"

	"github.com/kyverno/kyverno/ext/resource/convert"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	corev1 "k8s.io/api/core/v1"
)

type clientBasedResolver struct {
	client dclient.Interface
}

func NewConfigMapResolver(client dclient.Interface) (engineapi.ConfigmapResolver, error) {
	if client == nil {
		return nil, errors.New("client must not be nil")
	}
	return &clientBasedResolver{client}, nil
}

func (c *clientBasedResolver) Get(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	res, err := c.client.GetResource(ctx, "v1", "ConfigMap", namespace, name)
	if err != nil {
		return nil, err
	}

	return convert.To[corev1.ConfigMap](*res)
}
