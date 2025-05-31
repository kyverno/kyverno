package gpol

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"go.uber.org/multierr"
)

// CELGenerateController is used to process URs that are generated as a result of an event from the trigger resource.
type CELGenerateController struct {
	client        dclient.Interface
	context       libs.Context
	gpolEngine    gpolengine.Engine
	statusControl common.StatusControlInterface

	log logr.Logger
}

// NewCELGenerateController creates a new CELGenerateController.
func NewCELGenerateController(
	client dclient.Interface,
	context libs.Context,
	gpolEngine gpolengine.Engine,
	statusControl common.StatusControlInterface,
	log logr.Logger,
) *CELGenerateController {
	return &CELGenerateController{
		client:        client,
		context:       context,
		gpolEngine:    gpolEngine,
		statusControl: statusControl,
		log:           log,
	}
}

func (c *CELGenerateController) ProcessUR(ur *kyvernov2.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey())
	generatedResources := make([]kyvernov1.ResourceSpec, 0)
	logger.V(2).Info("start processing UR", "ur", ur.Name, "resourceVersion", ur.GetResourceVersion())

	var failures []error
	for i := 0; i < len(ur.Spec.RuleContext); i++ {
		trigger, err := common.GetTrigger(c.client, ur.Spec, i, c.log)
		if err != nil || trigger == nil {
			logger.V(4).Info("the trigger resource does not exist or is pending creation")
			failures = append(failures, fmt.Errorf("gpol %s failed: failed to fetch trigger resource: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		request := celengine.RequestFromAdmission(c.context, *ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest)
		err = c.gpolEngine.Generate(request, ur.Spec.GetPolicyKey())
		if err != nil {
			logger.Error(err, "failed to generate resources for gpol", "gpol", ur.Spec.GetPolicyKey())
			failures = append(failures, fmt.Errorf("gpol %s failed: %v", ur.Spec.GetPolicyKey(), err))
		}
	}
	return updateURStatus(c.statusControl, *ur, multierr.Combine(failures...), generatedResources)
}

func updateURStatus(statusControl common.StatusControlInterface, ur kyvernov2.UpdateRequest, err error, genResources []kyvernov1.ResourceSpec) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), genResources); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), genResources); err != nil {
			return err
		}
	}
	return nil
}
