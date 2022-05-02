package controllers

type Controller interface {
	Run(stopCh <-chan struct{})
}
