package checker

import (
	"strconv"

	"github.com/golang/glog"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
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
	client *dclient.Client
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
func NewVerifyControl(client *dclient.Client) *StatusControl {
	return &StatusControl{
		client: client,
	}
}

func (vc StatusControl) setStatus(status string) error {
	glog.Infof("setting deployment %s in ns %s annotation %s to %s", deployName, deployNamespace, annWebhookStats, status)
	var ann map[string]string
	var err error
	deploy, err := vc.client.GetResource("Deployment", deployNamespace, deployName)
	if err != nil {
		glog.V(4).Infof("failed to get deployment %s in namespace %s: %v", deployName, deployNamespace, err)
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
			glog.V(4).Infof("annotation %s already set to '%s'", annWebhookStats, status)
			return nil
		}
	}
	// set the status
	ann[annWebhookStats] = status
	deploy.SetAnnotations(ann)
	// update counter
	_, err = vc.client.UpdateResource("Deployment", deployNamespace, deploy, false)
	if err != nil {
		glog.V(4).Infof("failed to update annotation %s for deployment %s in namespace %s: %v", annWebhookStats, deployName, deployNamespace, err)
		return err
	}
	return nil
}

//IncrementAnnotation ...
func (vc StatusControl) IncrementAnnotation() error {
	glog.Infof("setting deployment %s in ns %s annotation %s", deployName, deployNamespace, annCounter)
	var ann map[string]string
	var err error
	deploy, err := vc.client.GetResource("Deployment", deployNamespace, deployName)
	if err != nil {
		glog.V(4).Infof("failed to get deployment %s in namespace %s: %v", deployName, deployNamespace, err)
		return err
	}
	ann = deploy.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
		ann[annCounter] = "0"
	}
	counter, err := strconv.Atoi(ann[annCounter])
	if err != nil {
		glog.V(4).Infof("failed to parse string: %v", err)
		return err
	}
	// increment counter
	counter++
	ann[annCounter] = strconv.Itoa(counter)
	glog.Infof("incrementing annotation %s counter to %d", annCounter, counter)
	deploy.SetAnnotations(ann)
	// update counter
	_, err = vc.client.UpdateResource("Deployment", deployNamespace, deploy, false)
	if err != nil {
		glog.V(4).Infof("failed to update annotation %s for deployment %s in namespace %s: %v", annCounter, deployName, deployNamespace, err)
		return err
	}
	return nil
}
