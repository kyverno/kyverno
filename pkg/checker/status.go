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
const annWebhookStatus string = "kyverno.io/webhookActive"

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
	logger := vc.log.WithValues("name", deployName, "namespace", deployNamespace)
	var ann map[string]string
	var err error
	deploy, err := vc.client.GetResource("Deployment", deployNamespace, deployName)
	if err != nil {
		logger.Error(err, "failed to get deployment")
		return err
	}

	ann = deploy.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
		ann[annWebhookStatus] = status
	}

	deployStatus, ok := ann[annWebhookStatus]
	if ok {
		// annotatiaion is present
		if deployStatus == status {
			logger.V(4).Info(fmt.Sprintf("annotation %s already set to '%s'", annWebhookStatus, status))
			return nil
		}
	}

	// set the status
	logger.Info("updating deployment annotation", "key", annWebhookStatus, "val", status)
	ann[annWebhookStatus] = status
	deploy.SetAnnotations(ann)

	// update counter
	_, err = vc.client.UpdateResource("Deployment", deployNamespace, deploy, false)
	if err != nil {
		logger.Error(err, "failed to update deployment annotation", "key", annWebhookStatus, "val", status)
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
	var ann map[string]string
	var err error
	deploy, err := vc.client.GetResource("Deployment", deployNamespace, deployName)
	if err != nil {
		logger.Error(err, "failed to find Kyverno", "deploymeny", deployName, "namespace", deployNamespace)
		return err
	}

	ann = deploy.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}

	if ann[annCounter] == "" {
		ann[annCounter] = "0"
	}

	counter, err := strconv.Atoi(ann[annCounter])
	if err != nil {
		logger.Error(err, "Failed to parse string", "name", annCounter, "value", ann[annCounter])
		return err
	}

	// increment counter
	counter++
	ann[annCounter] = strconv.Itoa(counter)

	logger.V(3).Info("updating webhook test annotation", "key", annCounter, "value", counter, "deployment", deployName, "namespace", deployNamespace)
	deploy.SetAnnotations(ann)

	// update counter
	_, err = vc.client.UpdateResource("Deployment", deployNamespace, deploy, false)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to update annotation %s for deployment %s in namespace %s", annCounter, deployName, deployNamespace))
		return err
	}

	return nil
}
