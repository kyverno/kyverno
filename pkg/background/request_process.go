package background

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
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

func (c *Controller) processUR(ur *kyvernov1beta1.UpdateRequest) error {
	switch ur.Spec.Type {
	case kyvernov1beta1.Mutate:
		ctrl, _ := mutate.NewMutateExistingController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.log, c.Config)
		return ctrl.ProcessUR(ur)

	case kyvernov1beta1.Generate:
		ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.nsLister, c.log, c.Config,
		)
		return ctrl.ProcessUR(ur)
	}
	return nil
}

func (c *Controller) markUR(ur *kyvernov1beta1.UpdateRequest) (*kyvernov1beta1.UpdateRequest, bool, error) {
	ur = ur.DeepCopy()
	if ur.Status.Handler != "" {
		return ur, ur.Status.Handler == config.KyvernoPodName(), nil
	}
	err := retry.RetryOnConflict(common.DefaultRetry, func() error {
		var retryError error
		ur.Status.Handler = config.KyvernoPodName()
		ur, retryError = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
		return retryError
	})
	return ur, true, err
}

func (c *Controller) unmarkUR(ur *kyvernov1beta1.UpdateRequest) error {
	if _, err := c.patchHandler(ur, ""); err != nil {
		return err
	}
	if ur.Spec.Type == kyvernov1beta1.Mutate && ur.Status.State == kyvernov1beta1.Completed {
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	}
	return nil
}

func (c *Controller) patchHandler(ur *kyvernov1beta1.UpdateRequest, val string) (*kyvernov1beta1.UpdateRequest, error) {
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
