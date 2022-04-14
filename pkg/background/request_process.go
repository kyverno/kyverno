package background

import (
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/generate"
)

func (c *Controller) ProcessGR(gr *urkyverno.UpdateRequest) error {
	ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
		c.policyLister, c.npolicyLister, c.grLister, c.eventGen, c.dynamicInformer, c.log, c.Config,
	)
	return ctrl.ProcessGR(gr)
}
