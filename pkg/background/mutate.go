package background

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) handleMutatePolicyAbsence(ur *kyvernov1beta1.UpdateRequest) error {
	selector := &metav1.LabelSelector{
		MatchLabels: common.MutateLabelsSet(ur.Spec.Policy, nil),
	}
	return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).DeleteCollection(
		context.TODO(),
		metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)},
	)
}
