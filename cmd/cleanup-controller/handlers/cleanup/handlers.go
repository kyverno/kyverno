package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
)

type handlers struct {
	client dclient.Interface
}

func New(client dclient.Interface) *handlers {
	return &handlers{
		client: client,
	}
}

func (h *handlers) Cleanup(ctx context.Context, logger logr.Logger, name string, _ time.Time) error {
	logger.Info("cleaning up...")
	return nil
}
