package main

import (
	"context"

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

func (c controller) run(ctx context.Context, logger logr.Logger) {
	logger.Info("start controller...", "name", c.name)
	c.controller.Run(ctx, c.workers)
}
