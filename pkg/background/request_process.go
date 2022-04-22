package background

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/generate"
)

func (c *Controller) ProcessGR(gr *kyverno.GenerateRequest) error {
	ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
		c.policyLister, c.npolicyLister, c.grLister, c.eventGen, c.namespaceInformer, c.log, c.Config,
	)
	return ctrl.ProcessGR(gr)
}
