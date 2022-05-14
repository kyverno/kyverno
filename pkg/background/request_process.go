package background

import (
	"context"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/background/mutate"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) ProcessUR(ur *urkyverno.UpdateRequest) error {
	switch ur.Spec.Type {
	case urkyverno.Mutate:
		ctrl, _ := mutate.NewMutateExistingController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.log, c.Config)
		return ctrl.ProcessUR(ur)

	case urkyverno.Generate:
		ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.nsLister, c.log, c.Config,
		)
		return ctrl.ProcessUR(ur)
	}
	return nil
}

func (c *Controller) MarkUR(ur *urkyverno.UpdateRequest) (*urkyverno.UpdateRequest, bool, error) {
	handler := ur.Status.Handler
	if handler != "" {
		if handler != config.KyvernoPodName() {
			return nil, false, nil
		}
		return ur, true, nil
	}

	handler = config.KyvernoPodName()
	ur.Status.Handler = handler
	new, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
	return new, true, err
}

func (c *Controller) UnmarkUR(ur *urkyverno.UpdateRequest) error {
	newUR, err := c.urLister.Get(ur.Name)
	if err != nil {
		return err
	}

	newUR.Status.Handler = ""
	_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), newUR, metav1.UpdateOptions{})
	return err
}
