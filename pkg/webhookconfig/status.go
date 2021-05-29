package webhookconfig

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var deployName string = config.KyvernoDeploymentName
var deployNamespace string = config.KyvernoNamespace

const (
	annCounter         string = "kyverno.io/generationCounter"
	annWebhookStatus   string = "kyverno.io/webhookActive"
	annLastRequestTime string = "kyverno.io/last-request-time"
)

//statusControl controls the webhook status
type statusControl struct {
	register *Register
	eventGen event.Interface
	log      logr.Logger
}

//success ...
func (vc statusControl) success() error {
	return vc.setStatus("true")
}

//failure ...
func (vc statusControl) failure() error {
	return vc.setStatus("false")
}

// NewStatusControl creates a new webhook status control
func newStatusControl(register *Register, eventGen event.Interface, log logr.Logger) *statusControl {
	return &statusControl{
		register: register,
		eventGen: eventGen,
		log:      log,
	}
}

func (vc statusControl) setStatus(status string) error {
	logger := vc.log.WithValues("name", deployName, "namespace", deployNamespace)
	var ann map[string]string
	var err error
	deploy, err := vc.register.client.GetResource("", "Deployment", deployNamespace, deployName)
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
	_, err = vc.register.client.UpdateResource("", "Deployment", deployNamespace, deploy, false)
	if err != nil {
		return errors.Wrapf(err, "key %s, val %s", annWebhookStatus, status)
	}

	// create event on kyverno deployment
	createStatusUpdateEvent(status, vc.eventGen)
	return nil
}

func createStatusUpdateEvent(status string, eventGen event.Interface) {
	e := event.Info{}
	e.Kind = "Deployment"
	e.Namespace = deployNamespace
	e.Name = deployName
	e.Reason = "Update"
	e.Message = fmt.Sprintf("admission control webhook active status changed to %s", status)
	eventGen.Add(e)
}

//IncrementAnnotation ...
func (vc statusControl) IncrementAnnotation() error {
	logger := vc.log
	var ann map[string]string
	var err error
	deploy, err := vc.register.client.GetResource("", "Deployment", deployNamespace, deployName)
	if err != nil {
		logger.Error(err, "failed to find Kyverno", "deployment", deployName, "namespace", deployNamespace)
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
	_, err = vc.register.client.UpdateResource("", "Deployment", deployNamespace, deploy, false)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to update annotation %s for deployment %s in namespace %s", annCounter, deployName, deployNamespace))
		return err
	}

	return nil
}

func (vc statusControl) UpdateLastRequestTimestmap(new time.Time) error {
	_, deploy, err := vc.register.GetKubePolicyDeployment()
	if err != nil {
		return errors.Wrap(err, "unable to get Kyverno deployment")
	}

	annotation, ok, err := unstructured.NestedStringMap(deploy.UnstructuredContent(), "metadata", "annotations")
	if err != nil {
		return errors.Wrap(err, "unable to get annotation")
	}

	if !ok {
		annotation = make(map[string]string)
	}

	t, err := new.MarshalText()
	if err != nil {
		return errors.Wrap(err, "failed to marshal timestamp")
	}

	annotation[annLastRequestTime] = string(t)
	deploy.SetAnnotations(annotation)
	_, err = vc.register.client.UpdateResource("", "Deployment", deploy.GetNamespace(), deploy, false)
	if err != nil {
		return errors.Wrapf(err, "failed to update annotation %s for deployment %s in namespace %s", annLastRequestTime, deploy.GetName(), deploy.GetNamespace())
	}

	return nil
}
