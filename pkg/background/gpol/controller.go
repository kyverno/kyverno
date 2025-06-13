package gpol

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CELGenerateController is used to process URs that are generated as a result of an event from the trigger resource.
type CELGenerateController struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	context  libs.Context
	engine   gpolengine.Engine
	provider gpolengine.Provider

	statusControl common.StatusControlInterface

	reportsConfig  reportutils.ReportingConfiguration
	reportsBreaker breaker.Breaker

	log logr.Logger
}

// NewCELGenerateController creates a new CELGenerateController.
func NewCELGenerateController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	context libs.Context,
	engine gpolengine.Engine,
	provider gpolengine.Provider,
	statusControl common.StatusControlInterface,
	reportsConfig reportutils.ReportingConfiguration,
	reportsBreaker breaker.Breaker,
	log logr.Logger,
) *CELGenerateController {
	return &CELGenerateController{
		client:         client,
		kyvernoClient:  kyvernoClient,
		context:        context,
		engine:         engine,
		provider:       provider,
		statusControl:  statusControl,
		reportsConfig:  reportsConfig,
		reportsBreaker: reportsBreaker,
		log:            log,
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
		policy, err := c.provider.Get(context.TODO(), ur.Spec.GetPolicyKey())
		if err != nil {
			logger.Error(err, "failed to fetch gpol", "gpol", ur.Spec.GetPolicyKey())
			failures = append(failures, fmt.Errorf("gpol %s failed: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		gpolResponse, err := c.engine.Handle(request, policy)
		if err != nil {
			logger.Error(err, "failed to generate resources for gpol", "gpol", ur.Spec.GetPolicyKey())
			failures = append(failures, fmt.Errorf("gpol %s failed: %v", ur.Spec.GetPolicyKey(), err))
		}
		engineResponse := engineapi.EngineResponse{
			Resource:       *gpolResponse.Trigger,
			PolicyResponse: engineapi.PolicyResponse{},
		}
		for _, res := range gpolResponse.Policies {
			for _, resource := range res.Result.GeneratedResources() {
				generatedResources = append(generatedResources, kyvernov1.ResourceSpec{
					Kind:       resource.GetKind(),
					APIVersion: resource.GetAPIVersion(),
					Name:       resource.GetName(),
					Namespace:  resource.GetNamespace(),
				})
			}
			engineResponse.PolicyResponse.Rules = []engineapi.RuleResponse{*res.Result}
			engineResponse = engineResponse.WithPolicy(engineapi.NewGeneratingPolicy(&res.Policy))
		}
		// generate reports if enabled
		if c.reportsConfig.GenerateReportsEnabled() {
			if err := c.createReports(context.TODO(), *trigger, engineResponse); err != nil {
				c.log.Error(err, "failed to create report")
			}
		}
	}
	return updateURStatus(c.statusControl, *ur, multierr.Combine(failures...), generatedResources)
}

func (c *CELGenerateController) createReports(
	ctx context.Context,
	resource unstructured.Unstructured,
	engineResponses ...engineapi.EngineResponse,
) error {
	report := reportutils.BuildGenerateReport(resource.GetNamespace(), resource.GroupVersionKind(), resource.GetName(), resource.GetUID(), engineResponses...)
	if len(report.GetResults()) > 0 {
		err := c.reportsBreaker.Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateReport(ctx, report, c.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
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
