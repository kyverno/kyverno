package checker

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
)

const deployName string = "kyverno"
const deployNamespace string = "kyverno"

const annCounter string = "kyverno.io/generationCounter"
const annWebhookStats string = "kyverno.io/webhookActive"

//StatusInterface provides api to update webhook active annotations on kyverno deployments
type StatusInterface interface {
	// Increments generation counter annotation
	IncrementAnnotation() error
	// update annotation to inform webhook is active
	SuccessStatus() error
	// update annotation to inform webhook is inactive
	FailedStatus() error
}

//StatusControl controls the webhook status
type StatusControl struct {
	client   *dclient.Client
	eventGen event.Interface
	log      logr.Logger
}

//SuccessStatus ...
func (vc StatusControl) SuccessStatus() error {
	return vc.setStatus("true")
}

//FailedStatus ...
func (vc StatusControl) FailedStatus() error {
	return vc.setStatus("false")
}

// NewVerifyControl ...
func NewVerifyControl(client *dclient.Client, eventGen event.Interface, log logr.Logger) *StatusControl {
	return &StatusControl{
		client:   client,
		eventGen: eventGen,
		log:      log,
	}
}

func (vc StatusControl) setStatus(status string) error {
	logger := vc.log
	logger.Info(fmt.Sprintf("setting deployment %s in ns %s annotation %s to %s", deployName, deployNamespace, annWebhookStats, status))
	var ann map[string]string
	var err error
	deploy, err := vc.client.GetResource("Deployment", deployNamespace, deployName)
	if err != nil {
		logger.Error(err, "failed to get deployment resource")
		return err
	}
	ann = deploy.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
		ann[annWebhookStats] = status
	}
	webhookAction, ok := ann[annWebhookStats]
	if ok {
		// annotatiaion is present
		if webhookAction == status {
			logger.V(4).Info(fmt.Sprintf("annotation %s already set to '%s'", annWebhookStats, status))
			return nil
		}
	}
	// set the status
	ann[annWebhookStats] = status
	deploy.SetAnnotations(ann)
	// update counter
	_, err = vc.client.UpdateResource("Deployment", deployNamespace, deploy, false)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to update annotation %s for deployment %s in namespace %s", annWebhookStats, deployName, deployNamespace))
		return err
	}
	// create event on kyverno deployment
	createStatusUpdateEvent(status, vc.eventGen)
	return nil
}

func createStatusUpdateEvent(status string, eventGen event.Interface) {
	e := event.Info{}
	e.Kind = "Deployment"
	e.Namespace = "kyverno"
	e.Name = "kyverno"
	e.Reason = "Update"
	e.Message = fmt.Sprintf("admission control webhook active status changed to %s", status)
	eventGen.Add(e)
}

//IncrementAnnotation ...
func (vc StatusControl) IncrementAnnotation() error {
	logger := vc.log
	logger.Info(fmt.Sprintf("setting deployment %s in ns %s annotation %s", deployName, deployNamespace, annCounter))
	var ann map[string]string
	var err error
	deploy, err := vc.client.GetResource("Deployment", deployNamespace, deployName)
	if err != nil {
		logger.Error(err, "failed to get deployment %s in namespace %s", deployName, deployNamespace)
		return err
	}
	ann = deploy.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
		ann[annCounter] = "0"
	}
	counter, err := strconv.Atoi(ann[annCounter])
	if err != nil {
		logger.Error(err, "Failed to parse string")
		return err
	}
	// increment counter
	counter++
	ann[annCounter] = strconv.Itoa(counter)
	logger.Info("incrementing annotation", "old", annCounter, "new", counter)
	deploy.SetAnnotations(ann)
	// update counter
	_, err = vc.client.UpdateResource("Deployment", deployNamespace, deploy, false)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to update annotation %s for deployment %s in namespace %s", annCounter, deployName, deployNamespace))
		return err
	}
	return nil
}
