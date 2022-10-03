package main

import (
	"context"

	"github.com/kyverno/kyverno/pkg/controllers"
)

type controller struct {
	controller controllers.Controller
	workers    int
}

func (c *controller) run(ctx context.Context) {
	c.controller.Run(ctx, c.workers)
}
