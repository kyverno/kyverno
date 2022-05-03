package controllers

type Controller interface {
	// Run starts the controller
	Run(stopCh <-chan struct{})
}
