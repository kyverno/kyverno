package main

import (
	"context"

	"github.com/kyverno/kyverno/pkg/controllers"
)

type controller struct {
	controller controllers.Controller
	workers    int
}

func newController(c controllers.Controller, w int) controller {
	return controller{
		controller: c,
		workers:    w,
	}
}

func (c *controller) start(ctx context.Context) {
	c.controller.Run(ctx, c.workers)
}
