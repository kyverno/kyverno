package controllers

import "context"

type Controller interface {
	// Run starts the controller
	Run(context.Context, int)
}
