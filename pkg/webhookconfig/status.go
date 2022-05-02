package webhookconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

const (
	leaseName          string = "kyverno"
	annWebhookStatus   string = "kyverno.io/webhookActive"
	annLastRequestTime string = "kyverno.io/last-request-time"
)

//statusControl controls the webhook status
type statusControl struct {
	eventGen    event.Interface
	log         logr.Logger
	leaseClient coordinationv1.LeaseInterface
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
func newStatusControl(leaseClient coordinationv1.LeaseInterface, eventGen event.Interface, log logr.Logger) *statusControl {
	return &statusControl{
		eventGen:    eventGen,
		log:         log,
		leaseClient: leaseClient,
	}
}

func (vc statusControl) setStatus(status string) error {
	logger := vc.log.WithValues("name", leaseName, "namespace", config.KyvernoNamespace)
	var ann map[string]string
	var err error

	lease, err := vc.leaseClient.Get(context.TODO(), "kyverno", metav1.GetOptions{})
	if err != nil {
		vc.log.WithName("UpdateLastRequestTimestmap").Error(err, "Lease 'kyverno' not found. Starting clean-up...")
		return err
	}

	ann = lease.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
		ann[annWebhookStatus] = status
	}

	leaseStatus, ok := ann[annWebhookStatus]
	if ok {
		if leaseStatus == status {
			logger.V(4).Info(fmt.Sprintf("annotation %s already set to '%s'", annWebhookStatus, status))
			return nil
		}
	}

	ann[annWebhookStatus] = status
	lease.SetAnnotations(ann)

	_, err = vc.leaseClient.Update(context.TODO(), lease, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "key %s, val %s", annWebhookStatus, status)
	}

	logger.Info("updated lease annotation", "key", annWebhookStatus, "val", status)

	// create event on kyverno deployment
	createStatusUpdateEvent(status, vc.eventGen)
	return nil
}

func createStatusUpdateEvent(status string, eventGen event.Interface) {
	e := event.Info{}
	e.Kind = "Lease"
	e.Namespace = config.KyvernoNamespace
	e.Name = leaseName
	e.Reason = "Update"
	e.Message = fmt.Sprintf("admission control webhook active status changed to %s", status)
	eventGen.Add(e)
}

func (vc statusControl) UpdateLastRequestTimestmap(new time.Time) error {

	lease, err := vc.leaseClient.Get(context.TODO(), leaseName, metav1.GetOptions{})
	if err != nil {
		vc.log.WithName("UpdateLastRequestTimestmap").Error(err, "Lease 'kyverno' not found. Starting clean-up...")
		return err
	}

	//add label to lease
	label := lease.GetLabels()
	if len(label) == 0 {
		label = make(map[string]string)
		label["app.kubernetes.io/name"] = "kyverno"
	}
	lease.SetLabels(label)

	annotation := lease.GetAnnotations()
	if annotation == nil {
		annotation = make(map[string]string)
	}

	t, err := new.MarshalText()
	if err != nil {
		return errors.Wrap(err, "failed to marshal timestamp")
	}

	annotation[annLastRequestTime] = string(t)
	lease.SetAnnotations(annotation)

	//update annotations in lease
	_, err = vc.leaseClient.Update(context.TODO(), lease, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to update annotation %s for deployment %s in namespace %s", annLastRequestTime, lease.GetName(), lease.GetNamespace())
	}

	return nil
}
