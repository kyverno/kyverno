package main

import (
	"context"

	"github.com/kyverno/kyverno/pkg/controllers"
)

type controller struct {
	controller controllers.Controller
	workers    int
	cancel     context.CancelFunc
}

func newController(c controllers.Controller, w int) controller {
	return controller{
		controller: c,
		workers:    w,
	}
}

func (c *controller) start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.controller.Run(ctx, c.workers)
}

func (c *controller) stop() {
	if c.cancel != nil {
		c.cancel()
	}
}
