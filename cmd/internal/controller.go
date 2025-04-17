package internal

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/controllers"
)

type Controller interface {
	Run(context.Context, logr.Logger, *sync.WaitGroup)
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

func (c controller) Run(ctx context.Context, logger logr.Logger, wg *sync.WaitGroup) {
	wg.Add(1)
	go func(logger logr.Logger) {
		logger.Info("starting controller", "workers", c.workers)
		defer logger.Info("controller stopped")
		defer wg.Done()
		c.controller.Run(ctx, c.workers)
	}(logger.WithValues("name", c.name))
}
