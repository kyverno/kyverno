package main

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/controllers"
)

type controller struct {
	name       string
	controller controllers.Controller
	workers    int
}

func newController(name string, c controllers.Controller, w int) controller {
	return controller{
		name:       name,
		controller: c,
		workers:    w,
	}
}

func (c controller) run(ctx context.Context, logger logr.Logger, wg *sync.WaitGroup) {
	wg.Add(1)
	go func(logger logr.Logger) {
		logger.Info("starting controller", "workers", c.workers)
		defer logger.Info("controller stopped")
		defer wg.Done()
		c.controller.Run(ctx, c.workers)
	}(logger.WithValues("name", c.name))
}
