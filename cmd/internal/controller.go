package internal

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/controllers"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Controller interface {
	Run(context.Context, logr.Logger, *wait.Group)
}

type controller struct {
	name       string
	controller controllers.Controller
	workers    int
}

func NewController(name string, c controllers.Controller, w int) Controller {
	return controller{
		name:       name,
		controller: c,
		workers:    w,
	}
}

func (c controller) Run(ctx context.Context, logger logr.Logger, wg *wait.Group) {
	logger = logger.WithValues("name", c.name)
	wg.Start(func() {
		logger.V(2).Info("starting controller", "workers", c.workers)
		defer logger.V(2).Info("controller stopped")
		c.controller.Run(ctx, c.workers)
	})
}
