package webhookconfig

import (
	"context"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

var deployName string = config.KyvernoDeploymentName
var deployNamespace string = config.KyvernoNamespace

const (
	annWebhookStatus   string = "kyverno.io/webhookActive"
	annLastRequestTime string = "kyverno.io/last-request-time"
)

//statusControl controls the webhook status
type statusControl struct {
	deployClient appsv1.DeploymentInterface
	eventGen     event.Interface
	log          logr.Logger
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
func newStatusControl(deployClient appsv1.DeploymentInterface, eventGen event.Interface, log logr.Logger) *statusControl {
	return &statusControl{
		deployClient: deployClient,
		eventGen:     eventGen,
		log:          log,
	}
}

func (vc statusControl) setStatus(status string) error {
	logger := vc.log.WithValues("name", deployName, "namespace", deployNamespace)
	var ann map[string]string
	var err error
	deploy, err := vc.deployClient.Get(context.TODO(), deployName, metav1.GetOptions{})
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

	ann[annWebhookStatus] = status
	deploy.SetAnnotations(ann)

	_, err = vc.deployClient.Update(context.TODO(), deploy, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "key %s, val %s", annWebhookStatus, status)
	}

	logger.Info("updated deployment annotation", "key", annWebhookStatus, "val", status)

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

func (vc statusControl) UpdateLastRequestTimestmap(new time.Time) error {
	deploy, err := vc.deployClient.Get(context.TODO(), deployName, metav1.GetOptions{})
	if err != nil {
		vc.log.WithName("UpdateLastRequestTimestmap").Error(err, "failed to get deployment")
		return err
	}

	annotation := deploy.GetAnnotations()
	if annotation == nil {
		annotation = make(map[string]string)
	}

	t, err := new.MarshalText()
	if err != nil {
		return errors.Wrap(err, "failed to marshal timestamp")
	}

	annotation[annLastRequestTime] = string(t)
	deploy.SetAnnotations(annotation)
	_, err = vc.deployClient.Update(context.TODO(), deploy, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to update annotation %s for deployment %s in namespace %s", annLastRequestTime, deploy.GetName(), deploy.GetNamespace())
	}

	return nil
}

// function to apply patch of last request time
func (vc statusControl) MergePatchLastRequestTimestmap(new time.Time) error {

	deploy, err := vc.deployClient.Get(context.TODO(), deployName, metav1.GetOptions{})
	if err != nil {
		vc.log.WithName("UpdateLastRequestTimestmap").Error(err, "failed to get deployment")
		return err
	}

	annotation := deploy.GetAnnotations()
	if annotation == nil {
		annotation = make(map[string]string)
	}

	t, err := new.MarshalText()
	if err != nil {
		return errors.Wrap(err, "failed to marshal timestamp")
	}

	annotation[annLastRequestTime] = string(t)
	deploy.SetAnnotations(annotation)
	response, err := jsonpatch.CreateMergePatch([]byte(`{}`), []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, annLastRequestTime, new.Format(time.RFC3339))))
	if err != nil {
		vc.log.WithName("MergePatchLastRequestTimestmap").Error(err, "failed to create merge patch")
		return nil
	}

	//apply patch
	_, err = vc.deployClient.Patch(context.TODO(), deployName, types.MergePatchType, response, metav1.PatchOptions{})
	if err != nil {
		vc.log.WithName("MergePatchLastRequestTimestmap").Error(err, "failed to apply patch")
		return nil
	}

	return nil
}

// function to apply patch of set status
func (vc statusControl) MergePatchSetStatus(status string) error {

	logger := vc.log.WithValues("name", deployName, "namespace", deployNamespace)
	var ann map[string]string
	var err error
	deploy, err := vc.deployClient.Get(context.TODO(), deployName, metav1.GetOptions{})
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

	ann[annWebhookStatus] = status
	deploy.SetAnnotations(ann)

	response, err := jsonpatch.CreateMergePatch([]byte(`{}`), []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, annWebhookStatus, status)))
	if err != nil {
		vc.log.WithName("MergePatchSetStatus").Error(err, "failed to create merge patch")
		return nil
	}

	//apply patch
	_, err = vc.deployClient.Patch(context.TODO(), deployName, types.MergePatchType, response, metav1.PatchOptions{})
	if err != nil {
		vc.log.WithName("MergePatchSetStatus").Error(err, "failed to apply patch")
		return nil
	}

	return nil
}
