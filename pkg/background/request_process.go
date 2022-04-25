package background

import (
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/background/mutate"
)

func (c *Controller) ProcessUR(ur *urkyverno.UpdateRequest) error {
	switch ur.Spec.Type {
	case urkyverno.Mutate:
		ctrl, _ := mutate.NewMutateExistingController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.log, c.Config)
		return ctrl.ProcessUR(ur)

	case urkyverno.Generate:
		ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.grLister, c.eventGen, c.nsLister, c.log, c.Config,
		)
		return ctrl.ProcessGR(ur)
	}
	return nil
}
