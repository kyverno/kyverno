package background

import (
	"context"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/background/mutate"
	"github.com/kyverno/kyverno/pkg/config"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
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
		if handler != config.KyvernoPodName {
			return nil, false, nil
		}
		return ur, true, nil
	}
	handler = config.KyvernoPodName
	ur.Status.Handler = handler
	var updateRequest *urkyverno.UpdateRequest

	err := retry.RetryOnConflict(common.DefaultRetry, func() error {
		var retryError error
		updateRequest, retryError = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return retryError
	})
	return updateRequest, true, err
}

func (c *Controller) UnmarkUR(ur *urkyverno.UpdateRequest) error {
	_, err := c.PatchHandler(ur, "")
	if err != nil {
		return err
	}

	if ur.Spec.Type == urkyverno.Mutate && ur.Status.State == urkyverno.Completed {
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	}
	return nil
}

func (c *Controller) PatchHandler(ur *urkyverno.UpdateRequest, val string) (*urkyverno.UpdateRequest, error) {
	patch := jsonutils.NewPatch(
		"/status/handler",
		"replace",
		val,
	)

	updateUR, err := common.PatchUpdateRequest(ur, patch, c.kyvernoClient, "status")
	if err != nil && !apierrors.IsNotFound(err) {
		c.log.Error(err, "failed to patch UpdateRequest: %v", patch)
		if val == "" {
			return nil, errors.Wrapf(err, "failed to patch UpdateRequest to clear /status/handler")
		}
		return nil, errors.Wrapf(err, "failed to patch UpdateRequest to update /status/handler to %s", val)
	}
	return updateUR, nil
}
