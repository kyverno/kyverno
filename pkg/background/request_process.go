package background

import (
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/generate"
)

func (c *Controller) ProcessUR(ur *urkyverno.UpdateRequest) error {
	switch ur.Spec.Type {
	case urkyverno.Mutate:
		// TODO (shuting): invoke mutate handler
	case urkyverno.Generate:
		ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.grLister, c.eventGen, c.dynamicInformer, c.log, c.Config,
		)
		return ctrl.ProcessGR(ur)
	}
	return nil
}
